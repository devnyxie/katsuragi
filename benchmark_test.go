package katsuragi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func largeHTMLWithLinks(n int) string {
	var b strings.Builder
	b.WriteString("<html><head><title>Large</title></head><body>")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<a href="/page%d">link %d</a>`, i, i)
	}
	b.WriteString("</body></html>")
	return b.String()
}

func BenchmarkRetrieveHTML_Cached(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Bench</title></head><body></body></html>`))
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	// warm the cache
	if _, err := f.GetTitle(context.Background(), server.URL); err != nil {
		b.Fatalf("warmup failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := f.GetTitle(context.Background(), server.URL); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkRetrieveHTML_Uncached(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Bench</title></head><body></body></html>`))
	}))
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
		if _, err := f.GetTitle(context.Background(), server.URL); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkGetLinks_LargeHTML(b *testing.B) {
	body := largeHTMLWithLinks(5000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.ClearCache()
		if _, err := f.GetLinks(context.Background(), GetLinksProps{Url: server.URL, Category: "all"}); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
