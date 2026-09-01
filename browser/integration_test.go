//go:build integration

// These tests drive a real Chrome/Chromium instance via chromedp and are
// separated behind the "integration" build tag because they require a
// browser binary on PATH (or CHROMEDP_TEST_EXEC_PATH set) and are slower
// than the fake-instance unit tests in pool_test.go. Run with:
//
//	go test -tags=integration ./browser/...
package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func newTestPool(t *testing.T, cfg PoolConfig) *Pool {
	t.Helper()
	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewPool failed (is Chrome/Chromium installed?): %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func serveFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "testdata", "html", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return string(body)
}

func TestIntegration_PoolAcquireAndNavigate(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 1, MaxTabsPerBrowser: 2})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>Hello</title></head><body></body></html>")
	}))
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	var title string
	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL), chromedp.Title(&title)); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if title != "Hello" {
		t.Fatalf("expected title %q, got %q", "Hello", title)
	}
}

func TestIntegration_MultiTabConcurrency(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 2, MaxTabsPerBrowser: 3})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>Multi</title></head><body></body></html>")
	}))
	defer server.Close()

	const n = 5
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			tab, err := pool.Acquire(ctx)
			if err != nil {
				errCh <- fmt.Errorf("acquire: %w", err)
				return
			}
			defer tab.Release()

			var title string
			if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL), chromedp.Title(&title)); err != nil {
				errCh <- err
				return
			}
			if title != "Multi" {
				errCh <- fmt.Errorf("expected title Multi, got %q", title)
				return
			}
			errCh <- nil
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent capture failed: %v", err)
		}
	}
}

func TestIntegration_NetworkIdleStrategy(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 1, MaxTabsPerBrowser: 1})

	fixture := serveFixture(t, "network_idle.html")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixture)
	})
	mux.HandleFunc("/slow-resource", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	cfg := WaitUntilNetworkIdle(5 * time.Second)
	if err := cfg.Prepare(tab.Ctx()); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL)); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	partial, err := cfg.Run(tab.Ctx())
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if partial {
		t.Fatalf("expected non-partial result once network settles")
	}

	var status string
	if err := chromedp.Run(tab.Ctx(), chromedp.Text("#status", &status)); err != nil {
		t.Fatalf("failed to read status: %v", err)
	}
	if status != "settled" {
		t.Fatalf("expected status %q after network-idle wait, got %q", "settled", status)
	}
}

func TestIntegration_LazyImageSettleStrategy(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 1, MaxTabsPerBrowser: 1})

	fixture := serveFixture(t, "lazy_images.html")
	pixel := []byte{ // 1x1 transparent PNG
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixture)
	})
	mux.HandleFunc("/pixel.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pixel)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	if err := chromedp.Run(tab.Ctx(),
		chromedp.EmulateViewport(1024, 768),
		chromedp.Navigate(server.URL),
	); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	cfg := WaitFullyLoaded(10 * time.Second)
	if _, err := cfg.Run(tab.Ctx()); err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	var loadedCount int
	const script = `document.querySelectorAll('img.lazy[src]').length`
	if err := chromedp.Run(tab.Ctx(), chromedp.Evaluate(script, &loadedCount)); err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if loadedCount != 3 {
		t.Fatalf("expected all 3 lazy images to have loaded after scroll, got %d", loadedCount)
	}
}

func TestIntegration_HangingPagePartialFallback(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 1, MaxTabsPerBrowser: 1})

	fixture := serveFixture(t, "hanging.html")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixture)
	})
	mux.HandleFunc("/never-responds", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // hold the connection open until the client gives up
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	cfg := WaitUntilNetworkIdle(2 * time.Second)
	if err := cfg.Prepare(tab.Ctx()); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL)); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	start := time.Now()
	partial, err := cfg.Run(tab.Ctx())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected partial success (no hard error) for a hanging page, got: %v", err)
	}
	if !partial {
		t.Fatalf("expected partial=true for a page that never reaches network idle")
	}
	if elapsed > 4*time.Second {
		t.Fatalf("expected Run to respect the overall timeout, took %s", elapsed)
	}
}

func TestIntegration_ChaosKillMidUse(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 2, MaxTabsPerBrowser: 1, HealthCheckInterval: 200 * time.Millisecond})

	// Kill one instance's underlying process out from under the pool,
	// bypassing chromedp's own shutdown path entirely.
	pool.mu.Lock()
	victim := pool.instances[0]
	pool.mu.Unlock()
	if victim.cmd == nil || victim.cmd.Process == nil {
		t.Fatalf("expected a real process handle on the victim instance")
	}
	if err := victim.cmd.Process.Kill(); err != nil {
		t.Fatalf("failed to kill victim process: %v", err)
	}

	// Give the health-check loop (running every 200ms) a chance to
	// detect the dead instance and recycle it.
	deadline := time.Now().Add(10 * time.Second)
	for {
		pool.mu.Lock()
		current := pool.instances[0]
		pool.mu.Unlock()
		if current != victim {
			break // recycled
		}
		if time.Now().After(deadline) {
			t.Fatalf("health check did not recycle the dead instance within the deadline")
		}
		time.Sleep(20 * time.Millisecond)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>Survivor</title></head><body></body></html>")
	}))
	defer server.Close()

	// The pool should still be usable end-to-end — no goroutine leaked
	// waiting on the dead instance's semaphore slot, no deadlock.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tab, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire failed after killing one instance: %v", err)
	}
	defer tab.Release()

	var title string
	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL), chromedp.Title(&title)); err != nil {
		t.Fatalf("navigate on recycled/surviving instance failed: %v", err)
	}
	if title != "Survivor" {
		t.Fatalf("expected title %q, got %q", "Survivor", title)
	}
}
