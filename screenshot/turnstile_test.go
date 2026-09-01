//go:build integration

package screenshot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/devnyxie/katsuragi/browser"
	"github.com/devnyxie/katsuragi/captcha"
)

// fakeSolver is a captcha.Solver that returns a fixed token without any
// network call, so detection/injection mechanics can be tested
// deterministically and without a paid solver account.
type fakeSolver struct {
	token       string
	err         error
	gotRequest  captcha.SolveRequest
	solveCalled bool
}

func (f *fakeSolver) Solve(ctx context.Context, req captcha.SolveRequest) (string, error) {
	f.solveCalled = true
	f.gotRequest = req
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

// turnstileFixture matches the documented Turnstile widget contract
// (https://developers.cloudflare.com/turnstile/get-started/client-side-rendering/):
// a .cf-turnstile element with data-sitekey/data-callback inside a <form>,
// plus a hidden cf-turnstile-response input the real widget would create.
// It's a synthetic stand-in for the real widget (which we can't drive
// deterministically without a paid solve or manual interaction), used to
// verify our own detection/injection code against the documented contract.
const turnstileFixture = `<html><body>
<form id="theform" action="/submit" method="post">
  <div class="cf-turnstile" data-sitekey="0x-test-sitekey" data-callback="onTurnstileSuccess"></div>
</form>
<script>
  window.callbackToken = null;
  window.onTurnstileSuccess = function(token) { window.callbackToken = token; };
</script>
</body></html>`

// turnstileFixtureNoForm is the same widget markup without an enclosing
// <form>, so solveTurnstile's injected script has nothing to submit and
// navigation away doesn't race with reading back window.callbackToken.
const turnstileFixtureNoForm = `<html><body>
<div class="cf-turnstile" data-sitekey="0x-test-sitekey" data-callback="onTurnstileSuccess"></div>
<script>
  window.callbackToken = null;
  window.onTurnstileSuccess = function(token) { window.callbackToken = token; };
</script>
</body></html>`

func TestIntegration_DetectTurnstile_FindsSiteKey(t *testing.T) {
	pool := newTestPool(t, browser.PoolConfig{Size: 1, MaxTabsPerBrowser: 1})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(turnstileFixture))
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

	siteKey, found, err := detectTurnstile(tab.Ctx())
	if err != nil {
		t.Fatalf("detectTurnstile failed: %v", err)
	}
	if !found {
		t.Fatalf("expected to find a Turnstile challenge, found none")
	}
	if siteKey != "0x-test-sitekey" {
		t.Fatalf("expected sitekey %q, got %q", "0x-test-sitekey", siteKey)
	}
}

func TestIntegration_DetectTurnstile_NoChallenge(t *testing.T) {
	pool := newTestPool(t, browser.PoolConfig{Size: 1, MaxTabsPerBrowser: 1})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body><h1>Normal page</h1></body></html>"))
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

	_, found, err := detectTurnstile(tab.Ctx())
	if err != nil {
		t.Fatalf("detectTurnstile failed: %v", err)
	}
	if found {
		t.Fatalf("expected no Turnstile challenge on a plain page")
	}
}

func TestIntegration_SolveTurnstile_TriggersCallback(t *testing.T) {
	pool := newTestPool(t, browser.PoolConfig{Size: 1, MaxTabsPerBrowser: 1})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(turnstileFixtureNoForm))
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

	siteKey, found, err := detectTurnstile(tab.Ctx())
	if err != nil || !found {
		t.Fatalf("expected to detect the challenge first, found=%v err=%v", found, err)
	}

	solver := &fakeSolver{token: "fake-solved-token"}
	if err := solveTurnstile(context.Background(), tab.Ctx(), solver, server.URL, siteKey); err != nil {
		t.Fatalf("solveTurnstile failed: %v", err)
	}

	if !solver.solveCalled {
		t.Fatalf("expected the solver to be called")
	}
	if solver.gotRequest.SiteKey != siteKey {
		t.Errorf("expected solver to receive sitekey %q, got %q", siteKey, solver.gotRequest.SiteKey)
	}
	if solver.gotRequest.Type != captcha.Turnstile {
		t.Errorf("expected solver request type %q, got %q", captcha.Turnstile, solver.gotRequest.Type)
	}

	var callbackToken string
	if err := chromedp.Run(tab.Ctx(), chromedp.Evaluate(`window.callbackToken`, &callbackToken)); err != nil {
		t.Fatalf("failed to read callback token: %v", err)
	}
	if callbackToken != "fake-solved-token" {
		t.Fatalf("expected data-callback to fire with the solved token, got %q", callbackToken)
	}
}

func TestIntegration_SolveTurnstile_SubmitsForm(t *testing.T) {
	pool := newTestPool(t, browser.PoolConfig{Size: 1, MaxTabsPerBrowser: 1})

	var mu sync.Mutex
	var submittedToken string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(turnstileFixture))
	})
	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		submittedToken = r.FormValue("cf-turnstile-response")
		mu.Unlock()
		_, _ = w.Write([]byte("<html><body>submitted</body></html>"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tab, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer tab.Release()

	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL)); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	siteKey, found, err := detectTurnstile(tab.Ctx())
	if err != nil || !found {
		t.Fatalf("expected to detect the challenge first, found=%v err=%v", found, err)
	}

	solver := &fakeSolver{token: "fake-solved-token"}
	if err := solveTurnstile(context.Background(), tab.Ctx(), solver, server.URL, siteKey); err != nil {
		t.Fatalf("solveTurnstile failed: %v", err)
	}

	// Wait for the form submission's navigation to complete.
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		got := submittedToken
		mu.Unlock()
		if got != "" || time.Now().After(deadline) {
			if got != "fake-solved-token" {
				t.Fatalf("expected the form to submit cf-turnstile-response=%q, got %q", "fake-solved-token", got)
			}
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestIntegration_SolveTurnstile_SolverError(t *testing.T) {
	pool := newTestPool(t, browser.PoolConfig{Size: 1, MaxTabsPerBrowser: 1})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(turnstileFixture))
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

	solver := &fakeSolver{err: context.DeadlineExceeded}
	err = solveTurnstile(context.Background(), tab.Ctx(), solver, server.URL, "0x-test-sitekey")
	if err == nil {
		t.Fatalf("expected an error when the solver fails, got none")
	}
}
