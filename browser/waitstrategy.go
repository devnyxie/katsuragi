package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// WaitStrategy is one stage of a layered "is this page ready" check. No
// single strategy reliably detects full page readiness across the range of
// sites a scraper encounters, so strategies are meant to be composed via
// WaitConfig rather than used alone.
type WaitStrategy interface {
	Name() string
	Wait(ctx context.Context) error
}

// Preparer is an optional interface a WaitStrategy can implement to set up
// state — enabling a CDP domain, attaching an event listener — before
// navigation begins. Strategies that only observe events firing after
// navigation (like NetworkIdleStrategy) would otherwise miss anything that
// happened during the page's initial load, since by the time Wait runs the
// navigation has already completed. Call WaitConfig.Prepare before
// navigating to give every such strategy a chance to hook in early.
type Preparer interface {
	Prepare(ctx context.Context) error
}

// DOMContentLoadedStrategy waits for a basic DOM element to exist. This is
// the cheapest, least reliable signal — a baseline, not a destination.
type DOMContentLoadedStrategy struct{}

func (DOMContentLoadedStrategy) Name() string { return "dom-content-loaded" }

func (DOMContentLoadedStrategy) Wait(ctx context.Context) error {
	return chromedp.Run(ctx, chromedp.WaitReady("body", chromedp.ByQuery))
}

// NetworkIdleStrategy waits until no network requests have been in flight
// for IdleDuration. This catches most XHR/fetch-driven SPA content that
// DOMContentLoaded misses.
//
// NetworkIdleStrategy must be used as a *NetworkIdleStrategy (not a value),
// and WaitConfig.Prepare must be called before navigating: it needs to
// enable the CDP Network domain and attach its event listener ahead of
// navigation, or it will miss requests that start during the initial page
// load — which, for most pages, is exactly the traffic it needs to see.
type NetworkIdleStrategy struct {
	// IdleDuration is how long the network must be quiet before the page
	// is considered settled. Defaults to 500ms.
	IdleDuration time.Duration

	state *networkIdleState
}

type networkIdleState struct {
	mu           sync.Mutex
	inflight     int
	lastActivity time.Time
}

func (*NetworkIdleStrategy) Name() string { return "network-idle" }

// Prepare enables the Network domain and attaches the request-tracking
// listener. See the type doc: call this (via WaitConfig.Prepare) before
// navigating.
func (s *NetworkIdleStrategy) Prepare(ctx context.Context) error {
	st := &networkIdleState{lastActivity: time.Now()}
	s.state = st

	chromedp.ListenTarget(ctx, func(ev any) {
		switch ev.(type) {
		case *network.EventRequestWillBeSent:
			st.mu.Lock()
			st.inflight++
			st.lastActivity = time.Now()
			st.mu.Unlock()
		case *network.EventLoadingFinished, *network.EventLoadingFailed:
			st.mu.Lock()
			if st.inflight > 0 {
				st.inflight--
			}
			st.lastActivity = time.Now()
			st.mu.Unlock()
		}
	})

	return chromedp.Run(ctx, network.Enable())
}

func (s *NetworkIdleStrategy) Wait(ctx context.Context) error {
	idle := s.IdleDuration
	if idle <= 0 {
		idle = 500 * time.Millisecond
	}

	st := s.state
	if st == nil {
		// Prepare was never called (e.g. this strategy is being used
		// standalone rather than through WaitConfig.Prepare) — fall back
		// to observing from here, accepting that anything before this
		// point is missed.
		if err := s.Prepare(ctx); err != nil {
			return err
		}
		st = s.state
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			st.mu.Lock()
			quiet := st.inflight == 0 && time.Since(st.lastActivity) >= idle
			st.mu.Unlock()
			if quiet {
				return nil
			}
		}
	}
}

// SelectorPresentStrategy waits for a caller-supplied CSS selector to become
// visible. This is the configurable custom-wait-condition escape hatch for
// callers who know the shape of a specific site.
type SelectorPresentStrategy struct {
	Selector string
}

func (SelectorPresentStrategy) Name() string { return "selector-present" }

func (s SelectorPresentStrategy) Wait(ctx context.Context) error {
	return chromedp.Run(ctx, chromedp.WaitVisible(s.Selector, chromedp.ByQuery))
}

// LazyImageSettleStrategy repeatedly scrolls toward the bottom of the page
// in steps (triggering intersection-observer/infinite-scroll driven loads)
// and checks how many <img> elements remain incomplete, stopping once that
// count reaches zero or stabilizes across iterations. It never returns an
// error — being unable to fully settle lazy content is expected on some
// pages, and the final FixedGraceStrategy stage (or PartialOnFailure) is
// what absorbs that.
type LazyImageSettleStrategy struct {
	// MaxIterations bounds how many scroll+check cycles are attempted.
	// Defaults to 10.
	MaxIterations int
	// StepDelay is how long to wait after each scroll before checking
	// image state. Defaults to 300ms.
	StepDelay time.Duration
}

func (LazyImageSettleStrategy) Name() string { return "lazy-image-settle" }

func (s LazyImageSettleStrategy) Wait(ctx context.Context) error {
	maxIter := s.MaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}
	delay := s.StepDelay
	if delay <= 0 {
		delay = 300 * time.Millisecond
	}

	// An <img> with no src attribute yet (the common lazy-load starting
	// state) is trivially img.complete === true per spec, so that alone
	// can't detect "still pending". naturalWidth stays 0 until the image
	// has actually decoded a frame, which is true both for a not-yet-
	// assigned src and for one that's still loading — a better proxy for
	// "not visibly settled" either way.
	const countIncompleteImagesJS = `Array.from(document.images).filter(img => img.naturalWidth === 0).length`

	// Deliberately no early-exit-on-no-progress here: a page with images
	// far below the fold will show an unchanged pending count across
	// several scroll steps before crossing the threshold that triggers
	// their IntersectionObserver, so a "stalled" reading is a normal
	// mid-scroll state, not necessarily a dead end.
	for i := 0; i < maxIter; i++ {
		frac := float64(i+1) / float64(maxIter)
		scrollJS := fmt.Sprintf(`window.scrollTo(0, document.body.scrollHeight * %f)`, frac)

		if err := chromedp.Run(ctx, chromedp.Evaluate(scrollJS, nil)); err != nil {
			return nil //nolint:nilerr // best-effort strategy, see doc comment
		}

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil //nolint:nilerr // best-effort strategy, see doc comment
		}

		var incomplete int
		if err := chromedp.Run(ctx, chromedp.Evaluate(countIncompleteImagesJS, &incomplete)); err != nil {
			return nil //nolint:nilerr // best-effort strategy, see doc comment
		}
		if incomplete == 0 {
			return nil
		}
	}
	return nil
}

// FixedGraceStrategy is a short fixed sleep, meant to run last, that catches
// last-moment paints CDP events miss. Keep it short — it is a pragmatic
// catch-all, not a substitute for the earlier stages.
type FixedGraceStrategy struct {
	// Duration is how long to sleep. Defaults to 300ms.
	Duration time.Duration
}

func (FixedGraceStrategy) Name() string { return "fixed-grace" }

func (s FixedGraceStrategy) Wait(ctx context.Context) error {
	d := s.Duration
	if d <= 0 {
		d = 300 * time.Millisecond
	}
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitConfig composes a sequence of WaitStrategy stages behind one overall
// deadline.
type WaitConfig struct {
	// Strategies run in order, each under its own sub-timeout carved out
	// of whatever time remains of OverallTimeout when that strategy
	// starts (split evenly across the strategies not yet run — so a
	// strategy that finishes early hands its unused time to the rest,
	// rather than each stage being capped at a fixed 1/N share
	// regardless of how the earlier stages went).
	Strategies []WaitStrategy
	// OverallTimeout bounds the total time spent waiting across all
	// strategies. Defaults to 30s.
	OverallTimeout time.Duration
	// PartialOnFailure, if true, makes Run report a partial result
	// instead of an error when any strategy in the chain fails to
	// complete cleanly (its own sub-timeout elapses, or the overall
	// deadline is exhausted) — signaling the caller to still capture
	// whatever rendered rather than aborting outright.
	PartialOnFailure bool
}

// Prepare runs Prepare on every strategy that implements Preparer. Call this
// before navigating so strategies that need to observe events from the very
// start of page load (like NetworkIdleStrategy) don't miss them.
func (c WaitConfig) Prepare(ctx context.Context) error {
	for _, s := range c.Strategies {
		if p, ok := s.(Preparer); ok {
			if err := p.Prepare(ctx); err != nil {
				return fmt.Errorf("browser: prepare wait strategy %q: %w", s.Name(), err)
			}
		}
	}
	return nil
}

// Run executes the configured strategies sequentially. A strategy that
// errors or times out is skipped in favor of the next one rather than
// aborting the whole chain — later, cheaper stages (like FixedGraceStrategy)
// are deliberately robust to earlier stages failing. Run reports
// partial=true whenever any strategy failed to complete cleanly and
// PartialOnFailure absorbed that instead of surfacing it as a hard error —
// the chain reaching its end doesn't by itself mean the page was confirmed
// ready.
func (c WaitConfig) Run(ctx context.Context) (partial bool, err error) {
	if len(c.Strategies) == 0 {
		return false, nil
	}

	overall := c.OverallTimeout
	if overall <= 0 {
		overall = 30 * time.Second
	}
	deadline := time.Now().Add(overall)
	overallCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	anyFailed := false

	for i, s := range c.Strategies {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			anyFailed = true
			break
		}

		budget := remaining / time.Duration(len(c.Strategies)-i)
		stratCtx, stratCancel := context.WithTimeout(overallCtx, budget)
		stratErr := s.Wait(stratCtx)
		stratCancel()

		if overallCtx.Err() != nil {
			anyFailed = true
			break
		}
		if stratErr != nil {
			anyFailed = true
			// non-fatal: absorbed, move on to the next stage
		}
	}

	if !anyFailed {
		return false, nil
	}
	if c.PartialOnFailure {
		return true, nil
	}
	return false, fmt.Errorf("browser: one or more wait strategies did not complete within the overall timeout")
}

// WaitUntilNetworkIdle is a convenience WaitConfig for the common case of
// waiting for DOM readiness plus network idle, with a short grace period.
func WaitUntilNetworkIdle(overallTimeout time.Duration) WaitConfig {
	return WaitConfig{
		Strategies: []WaitStrategy{
			DOMContentLoadedStrategy{},
			&NetworkIdleStrategy{},
			FixedGraceStrategy{},
		},
		OverallTimeout:   overallTimeout,
		PartialOnFailure: true,
	}
}

// WaitForSelector is a convenience WaitConfig for waiting on a
// caller-known CSS selector in addition to basic DOM readiness.
func WaitForSelector(selector string, overallTimeout time.Duration) WaitConfig {
	return WaitConfig{
		Strategies: []WaitStrategy{
			DOMContentLoadedStrategy{},
			SelectorPresentStrategy{Selector: selector},
			FixedGraceStrategy{},
		},
		OverallTimeout:   overallTimeout,
		PartialOnFailure: true,
	}
}

// WaitFullyLoaded is a convenience WaitConfig composing every layer: DOM
// readiness, network idle, lazy-image settling, and a final grace period.
// This is the most thorough (and slowest) preset, intended for pages known
// to be hard to render (heavy SPAs, infinite scroll, lazy images).
func WaitFullyLoaded(overallTimeout time.Duration) WaitConfig {
	return WaitConfig{
		Strategies: []WaitStrategy{
			DOMContentLoadedStrategy{},
			&NetworkIdleStrategy{},
			LazyImageSettleStrategy{},
			FixedGraceStrategy{},
		},
		OverallTimeout:   overallTimeout,
		PartialOnFailure: true,
	}
}
