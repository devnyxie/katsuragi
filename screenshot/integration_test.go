//go:build integration

package screenshot

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devnyxie/katsuragi/browser"
)

func newTestPool(t *testing.T, cfg browser.PoolConfig) *browser.Pool {
	t.Helper()
	pool, err := browser.NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewPool failed (is Chrome/Chromium installed?): %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestIntegration_CapturePNG(t *testing.T) {
	pool := newTestPool(t, browser.PoolConfig{Size: 1, MaxTabsPerBrowser: 2})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Shot</title></head><body style="background:red;height:100vh;"></body></html>`)
	}))
	defer server.Close()

	res, err := Capture(context.Background(), pool, Request{
		URL:        server.URL,
		WaitConfig: browser.WaitUntilNetworkIdle(5 * time.Second),
		Viewport:   Viewport{Width: 800, Height: 600},
	})
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}
	if len(res.ImageBytes) == 0 {
		t.Fatalf("expected non-empty image bytes")
	}
	if res.Partial {
		t.Fatalf("expected a clean (non-partial) result for a simple static page")
	}
	if _, err := png.Decode(bytes.NewReader(res.ImageBytes)); err != nil {
		t.Fatalf("expected valid PNG bytes: %v", err)
	}
}

func TestIntegration_CaptureFullPageJPEG(t *testing.T) {
	pool := newTestPool(t, browser.PoolConfig{Size: 1, MaxTabsPerBrowser: 2})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Tall</title></head><body style="height:3000px;background:blue;"></body></html>`)
	}))
	defer server.Close()

	res, err := Capture(context.Background(), pool, Request{
		URL:        server.URL,
		WaitConfig: browser.WaitUntilNetworkIdle(5 * time.Second),
		Viewport:   Viewport{Width: 800, Height: 600},
		FullPage:   true,
		Format:     FormatJPEG,
	})
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(res.ImageBytes))
	if err != nil {
		t.Fatalf("expected valid JPEG bytes: %v", err)
	}
	if img.Bounds().Dy() < 1000 {
		t.Fatalf("expected a tall full-page screenshot, got height %d", img.Bounds().Dy())
	}
}

func TestIntegration_CapturePartialOnHangingPage(t *testing.T) {
	pool := newTestPool(t, browser.PoolConfig{Size: 1, MaxTabsPerBrowser: 1})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Hangs</title></head><body><script>fetch('/never-responds')</script></body></html>`)
	})
	mux.HandleFunc("/never-responds", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := Capture(context.Background(), pool, Request{
		URL:        server.URL,
		WaitConfig: browser.WaitUntilNetworkIdle(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("expected a best-effort result, got error: %v", err)
	}
	if !res.Partial {
		t.Fatalf("expected Partial=true for a page that never settles")
	}
	if len(res.ImageBytes) == 0 {
		t.Fatalf("expected best-effort image bytes even though the page never settled")
	}
}
