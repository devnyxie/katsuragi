// Package screenshot captures rendered-page screenshots using a
// browser.Pool. It deliberately returns raw bytes and metadata only — where
// results are stored (disk, S3, a database) is a decision for the
// consuming application, not this library.
package screenshot

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/devnyxie/katsuragi/browser"
	"github.com/devnyxie/katsuragi/captcha"
)

// Viewport describes the browser viewport used for a capture.
type Viewport struct {
	Width             int64
	Height            int64
	DeviceScaleFactor float64
}

func (v Viewport) withDefaults() Viewport {
	if v.Width <= 0 {
		v.Width = 1280
	}
	if v.Height <= 0 {
		v.Height = 800
	}
	if v.DeviceScaleFactor <= 0 {
		v.DeviceScaleFactor = 1
	}
	return v
}

// Format is the image encoding to capture.
type Format string

const (
	FormatPNG  Format = "png"
	FormatJPEG Format = "jpeg"
)

// Request describes a single screenshot capture.
type Request struct {
	URL string

	// WaitConfig controls how long, and by what layered criteria, Capture
	// waits for the page to be considered ready before screenshotting.
	// See browser.WaitConfig and its convenience constructors
	// (browser.WaitUntilNetworkIdle, browser.WaitForSelector,
	// browser.WaitFullyLoaded).
	WaitConfig browser.WaitConfig

	Viewport Viewport
	FullPage bool
	// Format selects PNG vs JPEG encoding. Only honored when FullPage is
	// true: chromedp's viewport-only capture (used when FullPage is
	// false) always returns PNG.
	Format Format

	// AcquireTimeout bounds how long Capture waits for a free tab from
	// the pool before giving up. Defaults to 30s.
	AcquireTimeout time.Duration

	// CaptchaSolver, if set, makes Capture check for a Cloudflare
	// Turnstile challenge immediately after navigation and, if found,
	// solve it via the given Solver before proceeding to WaitConfig. Nil
	// (the default) skips this entirely — no behavior change for
	// existing callers. Solving a CAPTCHA this way has a real per-solve
	// cost and, on some sites, a ToS consideration worth weighing.
	CaptchaSolver captcha.Solver
}

// Result is the outcome of a Capture call.
type Result struct {
	ImageBytes []byte
	// Partial is true if the page did not fully satisfy its wait
	// strategy within the overall timeout and the screenshot reflects a
	// best-effort render rather than a fully-settled page.
	Partial    bool
	DurationMs int64
	// CaptchaSolved is true if Request.CaptchaSolver detected and solved
	// a Turnstile challenge during this capture.
	CaptchaSolved bool
}

// Capture acquires a tab from pool, navigates to req.URL, waits according to
// req.WaitConfig, takes a screenshot, and always releases the tab back to
// the pool before returning.
func Capture(ctx context.Context, pool *browser.Pool, req Request) (*Result, error) {
	start := time.Now()

	acquireTimeout := req.AcquireTimeout
	if acquireTimeout <= 0 {
		acquireTimeout = 30 * time.Second
	}
	acquireCtx, cancelAcquire := context.WithTimeout(ctx, acquireTimeout)
	tab, err := pool.Acquire(acquireCtx)
	cancelAcquire()
	if err != nil {
		return nil, fmt.Errorf("screenshot: failed to acquire browser tab: %w", err)
	}
	defer tab.Release()

	vp := req.Viewport.withDefaults()
	format := req.Format
	if format == "" {
		format = FormatPNG
	}

	if err := chromedp.Run(tab.Ctx(),
		chromedp.EmulateViewport(vp.Width, vp.Height, chromedp.EmulateScale(vp.DeviceScaleFactor)),
	); err != nil {
		return nil, fmt.Errorf("screenshot: failed to set viewport: %w", err)
	}

	// Strategies that need to observe events from the very start of page
	// load (e.g. NetworkIdleStrategy) must hook in before navigation, or
	// they'll miss requests fired during the initial load.
	if err := req.WaitConfig.Prepare(tab.Ctx()); err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}

	if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(req.URL)); err != nil {
		return nil, fmt.Errorf("screenshot: failed to navigate to %q: %w", req.URL, err)
	}

	var captchaSolved bool
	if req.CaptchaSolver != nil {
		siteKey, found, err := detectTurnstile(tab.Ctx())
		if err != nil {
			return nil, fmt.Errorf("screenshot: turnstile detection failed: %w", err)
		}
		if found {
			if err := solveTurnstile(ctx, tab.Ctx(), req.CaptchaSolver, req.URL, siteKey); err != nil {
				return nil, fmt.Errorf("screenshot: turnstile solve failed: %w", err)
			}
			captchaSolved = true
		}
	}

	partial, waitErr := req.WaitConfig.Run(tab.Ctx())
	if waitErr != nil {
		return nil, fmt.Errorf("screenshot: %w", waitErr)
	}

	var imgBytes []byte
	var shotErr error
	if req.FullPage {
		quality := 90
		if format == FormatPNG {
			quality = 100
		}
		shotErr = chromedp.Run(tab.Ctx(), chromedp.FullScreenshot(&imgBytes, quality))
	} else {
		shotErr = chromedp.Run(tab.Ctx(), chromedp.CaptureScreenshot(&imgBytes))
	}
	if shotErr != nil {
		return nil, fmt.Errorf("screenshot: capture failed: %w", shotErr)
	}

	return &Result{
		ImageBytes:    imgBytes,
		Partial:       partial,
		DurationMs:    time.Since(start).Milliseconds(),
		CaptchaSolved: captchaSolved,
	}, nil
}
