package browser

import "context"

// Tab wraps a single chromedp target (a browser tab) allocated from a Pool.
// Callers run chromedp actions against Ctx() and must call Release exactly
// once when done.
type Tab struct {
	ctx      context.Context
	cancel   context.CancelFunc
	instance *browserInstance
	pool     *Pool
}

// Ctx returns the chromedp context for this tab. Pass it to chromedp.Run.
func (t *Tab) Ctx() context.Context {
	return t.ctx
}

// Release closes the tab's target and returns its pool capacity, triggering
// a recycle check on the underlying browser instance if it is now idle and
// has crossed its use/age threshold. Release must be called exactly once
// per Tab, typically via defer immediately after Acquire succeeds.
func (t *Tab) Release() {
	t.cancel()
	t.pool.release(t.instance)
}
