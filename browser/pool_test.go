package browser

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newFakePool builds a Pool whose instances are fake (no real Chrome
// process), so bookkeeping — acquire/release, semaphore bounds, recycling,
// health handling — can be exercised quickly and without chromium installed.
func newFakePool(cfg PoolConfig) *Pool {
	cfg.setDefaults()
	p := &Pool{
		cfg:        cfg,
		sem:        make(chan struct{}, cfg.Size*cfg.MaxTabsPerBrowser),
		stopHealth: make(chan struct{}),
		activate:   func(context.Context, *browserInstance) error { return nil },
	}
	var n atomic.Int64
	p.launch = func(ctx context.Context) (*browserInstance, error) {
		id := n.Add(1)
		rootCtx, rootCancel := context.WithCancel(context.Background())
		inst := &browserInstance{
			id:          fmt.Sprintf("fake-%d", id),
			allocCtx:    rootCtx,
			allocCancel: rootCancel,
			rootCtx:     rootCtx,
			rootCancel:  func() {}, // separate no-op so allocCancel below is the one that actually cancels
			startedAt:   time.Now(),
		}
		inst.healthy.Store(true)
		return inst, nil
	}
	return p
}

func mustFillPool(t *testing.T, p *Pool, size int) {
	t.Helper()
	for i := 0; i < size; i++ {
		inst, err := p.launch(context.Background())
		if err != nil {
			t.Fatalf("fake launch failed: %v", err)
		}
		p.instances = append(p.instances, inst)
	}
}

func TestPool_AcquireRelease(t *testing.T) {
	p := newFakePool(PoolConfig{Size: 2, MaxTabsPerBrowser: 2})
	mustFillPool(t, p, 2)
	defer p.Close()

	tabs := make([]*Tab, 0, 4)
	for i := 0; i < 4; i++ {
		tab, err := p.Acquire(context.Background())
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
		tabs = append(tabs, tab)
	}

	stats := p.Stats()
	if stats.ActiveTabs != 4 {
		t.Fatalf("expected 4 active tabs, got %d", stats.ActiveTabs)
	}

	for _, tab := range tabs {
		tab.Release()
	}

	stats = p.Stats()
	if stats.ActiveTabs != 0 {
		t.Fatalf("expected 0 active tabs after release, got %d", stats.ActiveTabs)
	}
}

func TestPool_AcquireBlocksWhenSaturated(t *testing.T) {
	p := newFakePool(PoolConfig{Size: 1, MaxTabsPerBrowser: 1})
	mustFillPool(t, p, 1)
	defer p.Close()

	tab, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := p.Acquire(ctx); err == nil {
		t.Fatalf("expected Acquire to block/fail while pool saturated, got no error")
	}

	tab.Release()

	// Now a slot should be free.
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	tab2, err := p.Acquire(ctx2)
	if err != nil {
		t.Fatalf("expected Acquire to succeed after release, got: %v", err)
	}
	tab2.Release()
}

func TestPool_AcquireDistributesLoad(t *testing.T) {
	p := newFakePool(PoolConfig{Size: 2, MaxTabsPerBrowser: 3})
	mustFillPool(t, p, 2)
	defer p.Close()

	var tabs []*Tab
	for i := 0; i < 2; i++ {
		tab, err := p.Acquire(context.Background())
		if err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}
		tabs = append(tabs, tab)
	}

	// With 2 instances and 2 tabs acquired, load should be spread 1/1, not 2/0.
	for _, inst := range p.instances {
		if got := atomic.LoadInt32(&inst.activeTabs); got != 1 {
			t.Errorf("expected each instance to have 1 active tab, got %d", got)
		}
	}

	for _, tab := range tabs {
		tab.Release()
	}
}

func TestPool_ConcurrentAcquireRelease(t *testing.T) {
	p := newFakePool(PoolConfig{Size: 3, MaxTabsPerBrowser: 4})
	mustFillPool(t, p, 3)
	defer p.Close()

	const goroutines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			tab, err := p.Acquire(ctx)
			if err != nil {
				t.Errorf("Acquire failed: %v", err)
				return
			}
			time.Sleep(time.Millisecond)
			tab.Release()
		}()
	}
	wg.Wait()

	stats := p.Stats()
	if stats.ActiveTabs != 0 {
		t.Fatalf("expected 0 active tabs after all releases, got %d", stats.ActiveTabs)
	}
}

func TestPool_RecycleOnMaxUses(t *testing.T) {
	p := newFakePool(PoolConfig{Size: 1, MaxTabsPerBrowser: 1, MaxUsesPerBrowser: 2})
	mustFillPool(t, p, 1)
	defer p.Close()

	original := p.instances[0]

	for i := 0; i < 2; i++ {
		tab, err := p.Acquire(context.Background())
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
		tab.Release()
	}

	// Recycle is triggered asynchronously from release(); poll briefly.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		current := p.instances[0]
		p.mu.Unlock()
		if current != original {
			return // recycled successfully
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected instance to be recycled after MaxUsesPerBrowser reached")
}

func TestPool_UnhealthyInstanceExcludedFromAcquire(t *testing.T) {
	p := newFakePool(PoolConfig{Size: 2, MaxTabsPerBrowser: 1})
	mustFillPool(t, p, 2)
	defer p.Close()

	p.instances[0].healthy.Store(false)

	tab, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if tab.instance != p.instances[1] {
		t.Fatalf("expected the healthy instance to be picked, got a different one")
	}
	tab.Release()
}

// TestPool_CloseDoesNotLeakRacingRecycle is a regression test for a real
// bug: recycle() launches a replacement instance *after* releasing the
// pool lock, so a slow launch racing a concurrent Close() used to find
// p.instances already nilled out, silently drop the replacement on the
// floor, and leak its (real, in production) Chrome process forever. The
// fix is twofold: Close() now waits on recycleWG for any in-flight
// recycle to finish, and recycle() tears down a replacement it can't
// place back into p.instances instead of discarding it.
func TestPool_CloseDoesNotLeakRacingRecycle(t *testing.T) {
	p := newFakePool(PoolConfig{Size: 1, MaxTabsPerBrowser: 1, MaxUsesPerBrowser: 1})
	mustFillPool(t, p, 1)

	launchStarted := make(chan struct{})
	unblockLaunch := make(chan struct{})
	replacementTornDown := make(chan struct{})

	p.launch = func(ctx context.Context) (*browserInstance, error) {
		close(launchStarted)
		<-unblockLaunch // held open to simulate a slow (real) Chrome startup
		rootCtx, rootCancel := context.WithCancel(context.Background())
		inst := &browserInstance{id: "replacement", allocCtx: rootCtx, rootCtx: rootCtx, rootCancel: func() {}, startedAt: time.Now()}
		inst.allocCancel = func() {
			rootCancel()
			close(replacementTornDown)
		}
		inst.healthy.Store(true)
		return inst, nil
	}

	tab, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	tab.Release() // crosses MaxUsesPerBrowser, triggers spawnRecycle -> recycle() -> p.launch (blocks)

	select {
	case <-launchStarted:
	case <-time.After(time.Second):
		t.Fatalf("expected the recycle to start launching a replacement")
	}

	closeDone := make(chan struct{})
	go func() {
		p.Close()
		close(closeDone)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-closeDone:
		t.Fatalf("expected Close to block until the in-flight recycle finishes")
	default:
	}

	close(unblockLaunch)

	select {
	case <-replacementTornDown:
	case <-time.After(time.Second):
		t.Fatalf("expected the replacement launched by a recycle racing Close to be torn down, not leaked")
	}

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatalf("expected Close to return once the in-flight recycle finished")
	}
}

func TestPool_CloseIsIdempotentAndRejectsAcquire(t *testing.T) {
	p := newFakePool(PoolConfig{Size: 1, MaxTabsPerBrowser: 1})
	mustFillPool(t, p, 1)

	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second Close should be a no-op, got: %v", err)
	}

	if _, err := p.Acquire(context.Background()); err == nil {
		t.Fatalf("expected Acquire on a closed pool to fail")
	}
}
