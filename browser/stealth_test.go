//go:build integration

package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// fingerprint captures the specific values verified (against real Chrome)
// to differ between default and stealth-mode launches, plus the
// Notification.permission / permissions.query pair used to confirm stealth
// mode doesn't disturb their native (correct) relationship.
type fingerprint struct {
	userAgent          string
	webdriver          string
	notificationPerm   string
	notificationsState string
	webglVendor        string
	webglRenderer      string
}

func readFingerprint(t *testing.T, tabCtx context.Context) fingerprint {
	t.Helper()
	var fp fingerprint
	err := chromedp.Run(tabCtx,
		chromedp.Evaluate(`navigator.userAgent`, &fp.userAgent),
		chromedp.Evaluate(`String(navigator.webdriver)`, &fp.webdriver),
		chromedp.Evaluate(`Notification.permission`, &fp.notificationPerm),
		chromedp.Evaluate(
			`(async()=>{const r=await navigator.permissions.query({name:'notifications'});return r.state})()`,
			&fp.notificationsState,
			func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) },
		),
		chromedp.Evaluate(`(()=>{const c=document.createElement('canvas').getContext('webgl');const dbg=c.getExtension('WEBGL_debug_renderer_info');return c.getParameter(dbg.UNMASKED_VENDOR_WEBGL)})()`, &fp.webglVendor),
		chromedp.Evaluate(`(()=>{const c=document.createElement('canvas').getContext('webgl');const dbg=c.getExtension('WEBGL_debug_renderer_info');return c.getParameter(dbg.UNMASKED_RENDERER_WEBGL)})()`, &fp.webglRenderer),
	)
	if err != nil {
		t.Fatalf("failed to read fingerprint: %v", err)
	}
	return fp
}

func TestIntegration_Stealth_Off_ShowsKnownTells(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 1, MaxTabsPerBrowser: 1, Stealth: StealthOff})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL)); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	fp := readFingerprint(t, tab.Ctx())

	if !strings.Contains(fp.userAgent, "HeadlessChrome") {
		t.Errorf("expected default UA to contain 'HeadlessChrome', got %q", fp.userAgent)
	}
	if fp.webdriver != "true" {
		t.Errorf("expected default navigator.webdriver to be true, got %q", fp.webdriver)
	}
	if !strings.Contains(fp.webglRenderer, "SwiftShader") {
		t.Errorf("expected default WebGL renderer to mention SwiftShader (software rendering), got %q", fp.webglRenderer)
	}
	// Notification.permission="default" + permissions.query="prompt" is the
	// correct, spec-compliant native pairing (verified against a real page,
	// not about:blank) — not a headless tell. Asserted here as a baseline
	// so a regression in the "on" test below is caught.
	if fp.notificationPerm != "default" || fp.notificationsState != "prompt" {
		t.Errorf("expected native pairing default/prompt, got %q/%q", fp.notificationPerm, fp.notificationsState)
	}
}

func TestIntegration_Stealth_On_PatchesKnownTells(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 1, MaxTabsPerBrowser: 1, Stealth: StealthBasic, LaunchTimeout: 30 * time.Second})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL)); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	fp := readFingerprint(t, tab.Ctx())

	if strings.Contains(fp.userAgent, "Headless") {
		t.Errorf("expected stealth UA to not contain 'Headless', got %q", fp.userAgent)
	}
	if fp.webdriver != "false" {
		t.Errorf("expected stealth navigator.webdriver to be false, got %q", fp.webdriver)
	}
	// Stealth mode doesn't touch permissions.query — confirm it's still
	// the correct native pairing, not disturbed by the WebGL/UA patches.
	if fp.notificationPerm != "default" || fp.notificationsState != "prompt" {
		t.Errorf("expected native pairing default/prompt to be undisturbed by stealth mode, got %q/%q", fp.notificationPerm, fp.notificationsState)
	}
	if strings.Contains(fp.webglRenderer, "SwiftShader") {
		t.Errorf("expected stealth WebGL renderer to not mention SwiftShader, got %q", fp.webglRenderer)
	}
	if fp.webglVendor == "Google Inc. (Google)" {
		t.Errorf("expected stealth WebGL vendor to not be the software-rasterizer default, got %q", fp.webglVendor)
	}
}

func TestIntegration_Stealth_UAConsistentAcrossNavigations(t *testing.T) {
	pool := newTestPool(t, PoolConfig{Size: 1, MaxTabsPerBrowser: 1, Stealth: StealthBasic, LaunchTimeout: 30 * time.Second})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	var uaFirst, uaSecond string
	if err := chromedp.Run(tab.Ctx(),
		chromedp.Navigate(server.URL),
		chromedp.Evaluate(`navigator.userAgent`, &uaFirst),
		chromedp.Navigate(server.URL+"/again"),
		chromedp.Evaluate(`navigator.userAgent`, &uaSecond),
	); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	if uaFirst != uaSecond {
		t.Fatalf("expected UA override to persist across navigations on the same tab, got %q then %q", uaFirst, uaSecond)
	}
	if strings.Contains(uaFirst, "Headless") {
		t.Fatalf("expected patched UA, got %q", uaFirst)
	}
}
