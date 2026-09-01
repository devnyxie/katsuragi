//go:build integration

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devnyxie/katsuragi/browser"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	srv, err := New(context.Background(), Config{
		BrowserPoolSize: 1,
		BrowserMaxTabs:  1,
		Stealth:         browser.StealthOff,
	})
	if err != nil {
		t.Fatalf("New failed (is Chrome/Chromium installed?): %v", err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv
}

func TestIntegration_Healthz(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestIntegration_Extract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Hello</title><meta name="description" content="a page"></head><body></body></html>`))
	}))
	defer server.Close()

	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/extract?url="+server.URL, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body extractResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Title != "Hello" {
		t.Fatalf("expected title %q, got %q", "Hello", body.Title)
	}
	if body.Description != "a page" {
		t.Fatalf("expected description %q, got %q", "a page", body.Description)
	}
}

func TestIntegration_Screenshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body style="background:red"></body></html>`))
	}))
	defer server.Close()

	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/screenshot?url="+server.URL+"&width=200&height=150&timeoutMs=5000", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
	if w.Body.Len() == 0 {
		t.Fatalf("expected non-empty image bytes")
	}
}

func TestIntegration_ListenAndServe_GracefulShutdown(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.Port = 0 // ephemeral - avoids colliding with other processes on this host

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	// Give the listener a moment to bind.
	time.Sleep(200 * time.Millisecond)

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("expected ListenAndServe to return promptly after ctx cancellation")
	}
}
