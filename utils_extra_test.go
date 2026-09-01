package katsuragi

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRetrieveHTML_Gzip verifies that a gzip-compressed response (as
// signaled via Content-Encoding) is transparently decompressed by the HTTP
// client and parsed correctly.
func TestRetrieveHTML_Gzip(t *testing.T) {
	const body = `<html><head><title>Gzipped</title></head><body></body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		_, _ = gz.Write([]byte(body))
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	title, err := f.GetTitle(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if title != "Gzipped" {
		t.Fatalf("expected title %q, got %q", "Gzipped", title)
	}
}

// TestRetrieveHTML_Redirect verifies that a redirect chain is followed to
// its final destination and that page is fetched successfully.
func TestRetrieveHTML_Redirect(t *testing.T) {
	const body = `<html><head><title>Final</title></head><body></body></html>`

	var finalURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/middle", http.StatusFound)
	})
	mux.HandleFunc("/middle", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	})
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	finalURL = server.URL + "/start"

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	title, err := f.GetTitle(context.Background(), finalURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if title != "Final" {
		t.Fatalf("expected title %q, got %q", "Final", title)
	}
}

// TestCleanHtml_DeeplyNested guards against stack overflow/hangs when
// cleanHtml recurses through a pathologically deep or huge document.
func TestCleanHtml_DeeplyNested(t *testing.T) {
	const depth = 5000
	var b strings.Builder
	b.WriteString("<html><head><title>Deep</title></head><body>")
	for i := 0; i < depth; i++ {
		b.WriteString("<div>")
	}
	b.WriteString("<script>alert(1)</script>")
	for i := 0; i < depth; i++ {
		b.WriteString("</div>")
	}
	b.WriteString("</body></html>")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(b.String()))
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	title, err := f.GetTitle(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if title != "Deep" {
		t.Fatalf("expected title %q, got %q", "Deep", title)
	}
}

// TestFetcher_CacheRawBytes verifies the raw-bytes cache mode returns
// equivalent extraction results to the default parsed-node cache mode.
func TestFetcher_CacheRawBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Raw</title></head><body></body></html>`))
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10, CacheRawBytes: true})

	for i := 0; i < 3; i++ {
		title, err := f.GetTitle(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if title != "Raw" {
			t.Fatalf("expected title %q, got %q", "Raw", title)
		}
	}
}

// TestFetcher_Retry verifies that a transient 5xx failure is retried and
// eventually succeeds within MaxRetries.
func TestFetcher_Retry(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Recovered</title></head><body></body></html>`))
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10, MaxRetries: 3, RetryBackoff: 1})
	title, err := f.GetTitle(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("expected no error after retries, got: %v", err)
	}
	if title != "Recovered" {
		t.Fatalf("expected title %q, got %q", "Recovered", title)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// TestFetcher_RetryNonTransient verifies a non-transient error (e.g. 404)
// is not retried.
func TestFetcher_RetryNonTransient(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10, MaxRetries: 3, RetryBackoff: 1})
	_, err := f.GetTitle(context.Background(), server.URL)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt for a non-transient error, got %d", attempts)
	}
}
