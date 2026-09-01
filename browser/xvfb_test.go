//go:build integration && linux

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestIntegration_StealthHeadful_RealChromeNoHeadlessFlag(t *testing.T) {
	pool := newTestPool(t, PoolConfig{
		Size:              1,
		MaxTabsPerBrowser: 1,
		Stealth:           StealthHeadful,
		LaunchTimeout:     30 * time.Second,
	})

	pool.mu.Lock()
	inst := pool.instances[0]
	pool.mu.Unlock()

	if inst.xvfb == nil {
		t.Fatalf("expected a StealthHeadful instance to have an xvfbDisplay")
	}
	if inst.cmd == nil || inst.cmd.Process == nil {
		t.Fatalf("expected a captured Chrome process")
	}

	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", inst.cmd.Process.Pid))
	if err != nil {
		t.Fatalf("failed to read /proc/%d/cmdline: %v", inst.cmd.Process.Pid, err)
	}
	args := strings.Split(string(cmdline), "\x00")
	for _, a := range args {
		if strings.HasPrefix(a, "--headless") {
			t.Fatalf("expected no --headless argument in a StealthHeadful process, got args: %v", args)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	var visibility string
	if err := chromedp.Run(tab.Ctx(),
		chromedp.Navigate(server.URL),
		chromedp.Evaluate(`document.visibilityState`, &visibility),
	); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if visibility != "visible" {
		t.Fatalf("expected document.visibilityState to be 'visible', got %q", visibility)
	}
}

func TestIntegration_StealthHeadful_CaptureWorks(t *testing.T) {
	pool := newTestPool(t, PoolConfig{
		Size:              1,
		MaxTabsPerBrowser: 1,
		Stealth:           StealthHeadful,
		LaunchTimeout:     30 * time.Second,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><head><title>Headful</title></head><body></body></html>"))
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
	if title != "Headful" {
		t.Fatalf("expected title %q, got %q", "Headful", title)
	}
}
