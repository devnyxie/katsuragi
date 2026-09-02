// Package browser provides a pool of warm, ready-to-use headless Chrome
// instances built on chromedp, with multi-tab allocation, layered
// wait-for-load strategies, and lifecycle/health management.
//
// One Pool represents one process's worth of browsers. Horizontal scale is
// achieved by running multiple processes, each with its own Pool, behind
// whatever job distribution mechanism the consuming application uses — this
// package deliberately has no cross-process coordination. As a sizing
// rule of thumb, budget roughly 300-500MB of RSS per warm Chrome instance
// when choosing PoolConfig.Size relative to a container/host memory limit.
package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// PoolConfig configures a Pool of warm, ready-to-use headless Chrome
// instances.
type PoolConfig struct {
	// Size is the number of Chrome processes kept warm in the pool.
	Size int
	// MaxTabsPerBrowser bounds how many concurrent tabs (CDP targets) are
	// allocated within a single Chrome process.
	MaxTabsPerBrowser int
	// MaxUsesPerBrowser recycles a browser instance (kills and relaunches
	// it) once it has served this many tabs. 0 disables use-based
	// recycling.
	MaxUsesPerBrowser int64
	// MaxBrowserAge recycles a browser instance once it has been running
	// this long. 0 disables age-based recycling.
	MaxBrowserAge time.Duration
	// LaunchTimeout bounds how long a single Chrome process is given to
	// start up and respond to an initial navigation before launch is
	// considered failed.
	LaunchTimeout time.Duration
	// HealthCheckInterval controls how often idle instances are health
	// checked via a trivial navigation. 0 disables background health
	// checks.
	HealthCheckInterval time.Duration
	// ProcessKillGrace is how long to wait, after cancelling a browser
	// instance's context, before forcibly SIGKILLing its process in case
	// Chrome ignored the graceful shutdown request.
	ProcessKillGrace time.Duration
	// ExtraFlags are additional Chrome command-line flags (name -> value;
	// use true for boolean flags) merged on top of the pool's defaults
	// (headless, no-sandbox, disable-gpu, disable-dev-shm-usage).
	ExtraFlags map[string]interface{}

	// Stealth controls how hard this Pool tries to avoid looking
	// automated — see StealthLevel in stealth.go. Defaults to StealthOff
	// — existing callers see no behavior change.
	Stealth StealthLevel

	// Proxy, if set, routes every Chrome instance's traffic through the
	// given upstream proxy — see ProxyConfig in proxyauth.go.
	Proxy *ProxyConfig

	// Supervised, when true, launches Chrome under an intermediary
	// supervisor process instead of as a direct child, so an abrupt
	// death of this process (SIGKILL, OOM-kill, a crash) doesn't leave
	// Chrome orphaned — see supervisor.go for why this is necessary
	// (Pdeathsig alone was verified not to be reliable enough) and how
	// it works. Defaults to false — existing callers see no behavior
	// change. Requires os.Executable() to resolve to a re-runnable copy
	// of the current binary (true for normal builds; not for `go run`,
	// which compiles to a temp binary that's still present and
	// re-runnable in practice, just less obviously so).
	Supervised bool
}

func (c *PoolConfig) setDefaults() {
	if c.Size <= 0 {
		c.Size = 1
	}
	if c.MaxTabsPerBrowser <= 0 {
		c.MaxTabsPerBrowser = 5
	}
	if c.LaunchTimeout <= 0 {
		c.LaunchTimeout = 45 * time.Second
	}
	if c.HealthCheckInterval <= 0 {
		c.HealthCheckInterval = 30 * time.Second
	}
	if c.ProcessKillGrace <= 0 {
		c.ProcessKillGrace = 3 * time.Second
	}
}

// browserInstance is one warm Chrome process, potentially serving several
// concurrent tabs.
type browserInstance struct {
	id string

	allocCtx    context.Context
	allocCancel context.CancelFunc
	rootCtx     context.Context // keeps the browser alive; never used to run page actions directly
	rootCancel  context.CancelFunc
	cmd         *exec.Cmd // Chrome's own process, captured via chromedp.ModifyCmdFunc; nil in supervised mode

	// The following are set only when PoolConfig.Supervised is true (see
	// supervisor.go): Chrome is launched by supervisorCmd, not by this
	// package directly, so it can be Chrome's OS-level parent instead of
	// us. deathPipe must be closed in teardown to tell the supervisor to
	// kill Chrome and exit; userDataDir must be removed since supervised
	// mode bypasses chromedp's own temp-dir management.
	supervisorCmd *exec.Cmd
	deathPipe     *os.File
	userDataDir   string

	// userAgent is the stealth-mode "Headless"-stripped UA string for this
	// instance's real Chrome binary version, applied to every tab. Empty
	// when PoolConfig.Stealth is StealthOff.
	userAgent string

	// xvfb is the virtual display this instance's Chrome runs inside,
	// non-nil only for StealthHeadful (see xvfb_linux.go).
	xvfb *xvfbDisplay

	// proxy is a copy of PoolConfig.Proxy, carried per-instance so
	// activateTab doesn't need a reference back to the Pool. Nil unless
	// PoolConfig.Proxy was set.
	proxy *ProxyConfig

	activeTabs int32 // atomic
	uses       int64 // atomic
	startedAt  time.Time
	healthy    atomic.Bool
}

// Pool manages a fixed set of warm Chrome instances and hands out tabs
// (targets) bounded by a pool-wide concurrency limit.
type Pool struct {
	cfg PoolConfig

	mu        sync.Mutex
	instances []*browserInstance
	closed    bool

	sem chan struct{} // pool-wide semaphore, sized Size*MaxTabsPerBrowser

	stopHealth chan struct{}
	healthWG   sync.WaitGroup

	// recycleWG tracks in-flight recycle() goroutines (spawned from
	// release() and checkHealth()) so Close doesn't return - and the
	// process doesn't exit - while one is still mid-launch. Without this,
	// a recycle racing Close's instance-snapshot can finish launching a
	// replacement Chrome process after p.instances has already been read
	// and nilled out, leaving nothing referencing it: recycle's own
	// "place replacement back into p.instances" step silently no-ops
	// instead of finding a slot, and the process leaks as an orphan.
	recycleWG sync.WaitGroup

	nextID atomic.Int64

	// launch creates a new browser instance. It is p.launchChrome in
	// production; tests in this package substitute a fake so pool
	// bookkeeping (acquire/release, recycling, health) can be exercised
	// without spawning real Chrome processes.
	launch func(ctx context.Context) (*browserInstance, error)

	// activate is called once per newly acquired tab. It is
	// activateTab in production; tests in this package substitute a
	// no-op since fake instances have no real CDP target to activate.
	activate func(tabCtx context.Context, inst *browserInstance) error
}

// NewPool launches cfg.Size Chrome processes and returns a Pool ready to
// hand out tabs via Acquire. If any instance fails to launch, already-
// launched instances are torn down and an error is returned.
func NewPool(ctx context.Context, cfg PoolConfig) (*Pool, error) {
	cfg.setDefaults()

	p := &Pool{
		cfg:        cfg,
		sem:        make(chan struct{}, cfg.Size*cfg.MaxTabsPerBrowser),
		stopHealth: make(chan struct{}),
	}
	p.launch = p.launchChrome
	p.activate = activateTab

	for i := 0; i < cfg.Size; i++ {
		inst, err := p.launch(ctx)
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("browser: failed to launch instance %d/%d: %w", i+1, cfg.Size, err)
		}
		p.instances = append(p.instances, inst)
	}

	p.healthWG.Add(1)
	go p.healthLoop()

	return p, nil
}

func (p *Pool) launchChrome(ctx context.Context) (*browserInstance, error) {
	id := fmt.Sprintf("browser-%d", p.nextID.Add(1))
	inst := &browserInstance{id: id, startedAt: time.Now()}

	if p.cfg.Stealth == StealthHeadful {
		xvfb, err := startXvfb()
		if err != nil {
			return nil, fmt.Errorf("browser: failed to start Xvfb for StealthHeadful: %w", err)
		}
		inst.xvfb = xvfb
	}
	if p.cfg.Proxy != nil {
		inst.proxy = p.cfg.Proxy
	}

	var err error
	if p.cfg.Supervised {
		err = p.launchSupervised(ctx, inst)
	} else {
		err = p.launchViaExecAllocator(ctx, inst)
	}
	if err != nil {
		inst.xvfb.stop(p.cfg.ProcessKillGrace)
		return nil, err
	}

	inst.healthy.Store(true)

	if p.cfg.Stealth != StealthOff {
		ua, err := fetchAndStripUserAgent(inst.rootCtx)
		if err != nil {
			p.teardown(inst)
			return nil, fmt.Errorf("browser: failed to fetch user-agent for stealth mode: %w", err)
		}
		inst.userAgent = ua
	}

	return inst, nil
}

// launchViaExecAllocator is the default launch path: chromedp's
// ExecAllocator runs Chrome as a direct child of this process.
func (p *Pool) launchViaExecAllocator(ctx context.Context, inst *browserInstance) error {
	var opts []chromedp.ExecAllocatorOption
	if p.cfg.Stealth != StealthOff {
		opts = stealthExecOptions(p.cfg.Stealth != StealthHeadful)
	} else {
		opts = append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	}

	if inst.xvfb != nil {
		opts = append(opts, chromedp.Env(inst.xvfb.displayEnv(), inst.xvfb.authEnv()))
	}

	opts = append(opts,
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	if p.cfg.Proxy != nil {
		opts = append(opts,
			chromedp.ProxyServer(p.cfg.Proxy.Server),
			// Chrome bypasses the configured proxy for loopback
			// addresses by default (verified empirically) — meaning a
			// target that happens to resolve to 127.0.0.1/localhost
			// would silently skip the proxy entirely. Since the whole
			// point of configuring a proxy is that all traffic goes
			// through it, disable that implicit exception.
			chromedp.Flag("proxy-bypass-list", "<-loopback>"),
		)
	}
	for name, value := range p.cfg.ExtraFlags {
		opts = append(opts, chromedp.Flag(name, value))
	}

	opts = append(opts, chromedp.ModifyCmdFunc(func(cmd *exec.Cmd) {
		inst.cmd = cmd
		setDeathSignal(cmd)
	}))

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	rootCtx, rootCancel := chromedp.NewContext(allocCtx)

	// chromedp explicitly warns against wrapping the context used for the
	// *first* Run call (the one that allocates the browser) in a
	// context.WithTimeout: once that derived context's deadline elapses —
	// even long after a successful launch — chromedp tears down the whole
	// browser, since it was the allocating call. So the timeout here is
	// enforced externally instead, against the untouched rootCtx.
	launchErr := make(chan error, 1)
	go func() {
		launchErr <- chromedp.Run(rootCtx, chromedp.Navigate("about:blank"))
	}()

	select {
	case err := <-launchErr:
		if err != nil {
			rootCancel()
			allocCancel()
			return err
		}
	case <-time.After(p.cfg.LaunchTimeout):
		rootCancel()
		allocCancel()
		return fmt.Errorf("browser: launch timed out after %s", p.cfg.LaunchTimeout)
	case <-ctx.Done():
		rootCancel()
		allocCancel()
		return ctx.Err()
	}

	inst.allocCtx = allocCtx
	inst.allocCancel = allocCancel
	inst.rootCtx = rootCtx
	inst.rootCancel = rootCancel
	return nil
}

// launchSupervised is the PoolConfig.Supervised launch path: Chrome runs
// under an intermediary supervisor process (see supervisor.go) instead of
// as a direct child, so this pool's own process dying abruptly can't
// orphan it.
func (p *Pool) launchSupervised(ctx context.Context, inst *browserInstance) error {
	userDataDir, err := os.MkdirTemp("", "chromedp-runner")
	if err != nil {
		return fmt.Errorf("browser: failed to create user-data-dir: %w", err)
	}

	chromePath := findChromeExecPath()
	args := buildSupervisedArgs(p.cfg, userDataDir)

	var extraEnv []string
	if inst.xvfb != nil {
		extraEnv = []string{inst.xvfb.displayEnv(), inst.xvfb.authEnv()}
	}

	wsURL, supervisorCmd, deathPipe, err := launchSupervisedChrome(ctx, chromePath, args, extraEnv, p.cfg.LaunchTimeout)
	if err != nil {
		_ = os.RemoveAll(userDataDir)
		return err
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	rootCtx, rootCancel := chromedp.NewContext(allocCtx)

	launchErr := make(chan error, 1)
	go func() {
		launchErr <- chromedp.Run(rootCtx, chromedp.Navigate("about:blank"))
	}()

	fail := func(err error) error {
		rootCancel()
		allocCancel()
		_ = deathPipe.Close()
		_ = supervisorCmd.Process.Kill()
		_ = os.RemoveAll(userDataDir)
		return err
	}

	select {
	case err := <-launchErr:
		if err != nil {
			return fail(err)
		}
	case <-time.After(p.cfg.LaunchTimeout):
		return fail(fmt.Errorf("browser: launch timed out after %s", p.cfg.LaunchTimeout))
	case <-ctx.Done():
		return fail(ctx.Err())
	}

	inst.allocCtx = allocCtx
	inst.allocCancel = allocCancel
	inst.rootCtx = rootCtx
	inst.rootCancel = rootCancel
	inst.supervisorCmd = supervisorCmd
	inst.deathPipe = deathPipe
	inst.userDataDir = userDataDir
	return nil
}

// teardown cancels a browser instance's context (which asks chromedp/Chrome
// to shut down) and, if the process hasn't exited within ProcessKillGrace,
// forcibly kills it. Chrome occasionally ignores graceful shutdown signals,
// especially in containers, so this is a deliberate belt-and-suspenders step
// rather than trusting cancellation alone.
func (p *Pool) teardown(inst *browserInstance) {
	inst.healthy.Store(false)
	inst.rootCancel()
	inst.allocCancel()
	defer inst.xvfb.stop(p.cfg.ProcessKillGrace)

	if inst.deathPipe != nil {
		p.teardownSupervised(inst)
		return
	}

	if inst.cmd == nil || inst.cmd.Process == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		_, _ = inst.cmd.Process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(p.cfg.ProcessKillGrace):
		_ = inst.cmd.Process.Kill()
	}
}

// teardownSupervised tears down a PoolConfig.Supervised instance: closing
// deathPipe tells the supervisor process to SIGKILL Chrome and exit (see
// runSupervisor), which is the normal, expected path — the ProcessKillGrace
// wait/kill below is a fallback in case the supervisor itself doesn't exit
// promptly (e.g. it's stuck), and userDataDir is removed since supervised
// mode bypasses chromedp's own temp-dir cleanup.
func (p *Pool) teardownSupervised(inst *browserInstance) {
	_ = inst.deathPipe.Close()

	if inst.userDataDir != "" {
		defer os.RemoveAll(inst.userDataDir)
	}

	if inst.supervisorCmd == nil || inst.supervisorCmd.Process == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		_, _ = inst.supervisorCmd.Process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(p.cfg.ProcessKillGrace):
		_ = inst.supervisorCmd.Process.Kill()
	}
}

// Acquire blocks until a tab is available (bounded by the pool-wide
// semaphore) or ctx is done, then returns a Tab backed by the
// least-loaded healthy browser instance.
func (p *Pool) Acquire(ctx context.Context) (*Tab, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("browser: pool is closed")
	}
	p.mu.Unlock()

	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	inst, err := p.pickInstance()
	if err != nil {
		<-p.sem
		return nil, err
	}

	atomic.AddInt32(&inst.activeTabs, 1)
	atomic.AddInt64(&inst.uses, 1)

	tabCtx, tabCancel := chromedp.NewContext(inst.rootCtx)

	if err := p.activate(tabCtx, inst); err != nil {
		tabCancel()
		atomic.AddInt32(&inst.activeTabs, -1)
		<-p.sem
		return nil, fmt.Errorf("browser: failed to create/activate tab: %w", err)
	}

	return &Tab{
		ctx:      tabCtx,
		cancel:   tabCancel,
		instance: inst,
		pool:     p,
	}, nil
}

// activateTab forces the tab's underlying CDP target to be created (Run
// with zero actions still runs chromedp's "ensure a target exists" step)
// and then activates it.
//
// This matters because every tab after a pool instance's first is not the
// foreground target from Chrome's perspective: document.visibilityState
// reports "hidden" for it, and Chrome throttles/suppresses
// visibility-driven behavior on hidden pages — notably, IntersectionObserver
// callbacks never fire, which silently breaks lazy-loaded content (see
// LazyImageSettleStrategy) and can affect rAF-driven rendering more broadly.
// Explicitly activating each tab keeps it behaving like a normal foreground
// page regardless of how many other tabs share the same browser process.
//
// If inst.userAgent is set (PoolConfig.Stealth was on for this instance),
// it also applies the stealth UA override and fingerprint patches from
// stealth.go to this tab.
func activateTab(tabCtx context.Context, inst *browserInstance) error {
	if err := chromedp.Run(tabCtx); err != nil {
		return err
	}
	c := chromedp.FromContext(tabCtx)
	if c == nil || c.Target == nil || c.Browser == nil {
		return fmt.Errorf("tab has no target after creation")
	}
	if err := target.ActivateTarget(c.Target.TargetID).Do(cdp.WithExecutor(tabCtx, c.Browser)); err != nil {
		return err
	}
	if inst.userAgent != "" {
		if err := applyStealthToTab(tabCtx, inst.userAgent); err != nil {
			return err
		}
	}
	if inst.proxy.authenticated() {
		if err := enableProxyAuth(tabCtx, *inst.proxy); err != nil {
			return err
		}
	}
	return nil
}

// pickInstance returns the least-loaded healthy instance with capacity for
// another tab.
func (p *Pool) pickInstance() (*browserInstance, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var best *browserInstance
	for _, inst := range p.instances {
		if !inst.healthy.Load() {
			continue
		}
		if atomic.LoadInt32(&inst.activeTabs) >= int32(p.cfg.MaxTabsPerBrowser) {
			continue
		}
		if best == nil || atomic.LoadInt32(&inst.activeTabs) < atomic.LoadInt32(&best.activeTabs) {
			best = inst
		}
	}
	if best == nil {
		return nil, fmt.Errorf("browser: no healthy instance with capacity available")
	}
	return best, nil
}

// release is called by Tab.Release. It decrements bookkeeping, frees the
// pool-wide semaphore slot, and recycles the instance if it has crossed its
// use/age threshold and is now idle.
func (p *Pool) release(inst *browserInstance) {
	atomic.AddInt32(&inst.activeTabs, -1)
	<-p.sem

	if !p.shouldRecycle(inst) {
		return
	}
	p.spawnRecycle(inst)
}

// spawnRecycle starts a recycle() goroutine for inst, tracked by
// recycleWG so Close can wait for it. It's a no-op once the pool is
// closed - the closed check and the WaitGroup Add happen atomically
// under p.mu so no Add can race past a concurrent Close's Wait (see
// recycleWG's doc comment).
func (p *Pool) spawnRecycle(inst *browserInstance) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.recycleWG.Add(1)
	p.mu.Unlock()

	go func() {
		defer p.recycleWG.Done()
		p.recycle(inst)
	}()
}

func (p *Pool) shouldRecycle(inst *browserInstance) bool {
	if atomic.LoadInt32(&inst.activeTabs) != 0 {
		return false
	}
	if p.cfg.MaxUsesPerBrowser > 0 && atomic.LoadInt64(&inst.uses) >= p.cfg.MaxUsesPerBrowser {
		return true
	}
	if p.cfg.MaxBrowserAge > 0 && time.Since(inst.startedAt) > p.cfg.MaxBrowserAge {
		return true
	}
	return false
}

// recycle tears down inst and replaces it in the pool with a freshly
// launched instance. If relaunching fails, the slot is left unhealthy and
// excluded from Acquire until a future health check (or recycle) succeeds.
func (p *Pool) recycle(inst *browserInstance) {
	p.teardown(inst)

	replacement, err := p.launch(context.Background())

	p.mu.Lock()
	defer p.mu.Unlock()
	for i, existing := range p.instances {
		if existing == inst {
			if err == nil {
				p.instances[i] = replacement
			}
			// If relaunch failed, leave the old (now unhealthy, torn
			// down) instance in place; it will be retried by the next
			// health check tick or recycle attempt rather than
			// permanently shrinking the pool.
			return
		}
	}
	// inst is no longer in p.instances - the pool was closed while this
	// recycle's replacement was launching. Tear the replacement down
	// instead of silently dropping the only reference to it, which would
	// otherwise leak its Chrome process as an orphan.
	if err == nil {
		p.teardown(replacement)
	}
}

// healthLoop periodically verifies idle instances are still responsive,
// marking failures unhealthy (excluding them from Acquire) and recycling
// them.
func (p *Pool) healthLoop() {
	defer p.healthWG.Done()

	ticker := time.NewTicker(p.cfg.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopHealth:
			return
		case <-ticker.C:
			p.checkHealth()
		}
	}
}

func (p *Pool) checkHealth() {
	p.mu.Lock()
	instances := append([]*browserInstance{}, p.instances...)
	p.mu.Unlock()

	for _, inst := range instances {
		if atomic.LoadInt32(&inst.activeTabs) != 0 {
			continue // don't disturb an instance mid-use
		}
		ctx, cancel := context.WithTimeout(inst.rootCtx, 5*time.Second)
		err := chromedp.Run(ctx, chromedp.Navigate("about:blank"))
		cancel()
		if err != nil {
			inst.healthy.Store(false)
			p.spawnRecycle(inst)
		}
	}
}

// Close tears down every instance in the pool and stops background health
// checks. It does not wait for in-flight tabs to be released.
func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	instances := p.instances
	p.instances = nil
	p.mu.Unlock()

	close(p.stopHealth)
	p.healthWG.Wait()
	p.recycleWG.Wait()

	for _, inst := range instances {
		p.teardown(inst)
	}
	return nil
}

// Stats reports point-in-time pool utilization, useful for logging/metrics.
type Stats struct {
	Instances        int
	HealthyInstances int
	ActiveTabs       int
}

// Stats returns a snapshot of current pool utilization.
func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()

	s := Stats{Instances: len(p.instances)}
	for _, inst := range p.instances {
		if inst.healthy.Load() {
			s.HealthyInstances++
		}
		s.ActiveTabs += int(atomic.LoadInt32(&inst.activeTabs))
	}
	return s
}
