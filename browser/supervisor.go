package browser

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	supervisorEnvFlag = "KATSURAGI_SUPERVISOR"
	supervisorEnvSpec = "KATSURAGI_SUPERVISOR_SPEC"
)

// init intercepts a re-exec'd copy of the consuming binary before any of
// its own main() logic runs — Go guarantees every imported package's
// init() completes before main() starts — and, if this process was
// launched by launchSupervisedChrome, jumps straight into runSupervisor
// instead of letting the consumer's program run at all. This is the same
// "re-exec self as a helper" pattern used by e.g. Docker's pkg/reexec and
// is what makes PoolConfig.Supervised possible without a separately
// compiled/shipped binary.
func init() {
	if os.Getenv(supervisorEnvFlag) == "1" {
		runSupervisor()
		os.Exit(1) // runSupervisor always calls os.Exit itself; this is a safety net.
	}
}

type supervisorSpec struct {
	ChromePath string   `json:"chromePath"`
	Args       []string `json:"args"`
	Env        []string `json:"env"`
}

// runSupervisor is the entrypoint for the re-exec'd supervisor process. It
// never returns.
//
// Why this exists: there is no way for a process to react to its own
// SIGKILL — by definition, none of its code runs once that happens. The
// usual mitigation, Pdeathsig, asks the kernel to react on the process's
// behalf instead, but it's tied to the specific OS thread that made the
// syscall staying alive for the process's lifetime, which Go's scheduler
// doesn't guarantee (https://go.dev/issue/27505) — and it was verified
// empirically during this library's development not to save Chrome from
// being orphaned, even via chromedp's own built-in default use of it.
//
// The robust fix — the same one tini/dumb-init use as a container's PID 1
// — is a surviving intermediary process. This supervisor is that
// intermediary:
//  1. It launches Chrome as its own child, so it (not the original
//     katsuragi process) is Chrome's OS-level parent.
//  2. It blocks reading a pipe whose write end only the original process
//     holds open (fd 3, inherited via cmd.ExtraFiles). When that process
//     dies for any reason — including SIGKILL — the OS closes the write
//     end automatically, and this blocking read immediately unblocks.
//  3. The instant that happens, it SIGKILLs Chrome and exits.
//
// It also discovers Chrome's DevTools websocket URL from its output and
// reports it back to the original process over a second pipe (fd 4), so
// the original process can drive Chrome remotely via
// chromedp.NewRemoteAllocator — from that point on, Chrome being a
// grandchild of the original process rather than a direct child is
// invisible to the rest of this package.
func runSupervisor() {
	deathPipe := os.NewFile(3, "katsuragi-death-pipe")
	urlPipe := os.NewFile(4, "katsuragi-url-pipe")

	var spec supervisorSpec
	if err := json.Unmarshal([]byte(os.Getenv(supervisorEnvSpec)), &spec); err != nil {
		fmt.Fprintln(os.Stderr, "katsuragi supervisor: invalid spec:", err)
		os.Exit(1)
	}

	cmd := exec.Command(spec.ChromePath, spec.Args...)
	cmd.Env = spec.Env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "katsuragi supervisor: stdout pipe:", err)
		os.Exit(1)
	}
	cmd.Stderr = cmd.Stdout // Chrome prints "DevTools listening on ..." on stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "katsuragi supervisor: failed to start chrome:", err)
		os.Exit(1)
	}

	go func() {
		const prefix = "DevTools listening on"
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		reported := false
		for scanner.Scan() {
			if reported {
				continue // keep draining so Chrome never blocks on a full pipe
			}
			line := scanner.Text()
			if idx := strings.Index(line, prefix); idx >= 0 {
				wsURL := strings.TrimSpace(line[idx+len(prefix):])
				fmt.Fprintln(urlPipe, wsURL)
				_ = urlPipe.Close()
				reported = true
			}
		}
		if !reported {
			_ = urlPipe.Close()
		}
	}()

	go func() {
		buf := make([]byte, 1)
		_, _ = deathPipe.Read(buf) // blocks; unblocks when the write end closes (parent died)
		_ = cmd.Process.Kill()
		os.Exit(0)
	}()

	_ = cmd.Wait()
	os.Exit(0)
}

// launchSupervisedChrome launches Chrome under a supervisor process (see
// runSupervisor) and returns the DevTools websocket URL to connect to via
// chromedp.NewRemoteAllocator, the supervisor's *exec.Cmd (for
// wait/kill-grace bookkeeping in teardown), and the death-pipe writer that
// must be kept open for the lifetime of this browser instance — closing it
// tells the supervisor to kill Chrome and exit.
func launchSupervisedChrome(ctx context.Context, chromePath string, args []string, extraEnv []string, launchTimeout time.Duration) (wsURL string, supervisorCmd *exec.Cmd, deathPipeWriter *os.File, err error) {
	deathR, deathW, err := os.Pipe()
	if err != nil {
		return "", nil, nil, fmt.Errorf("browser: failed to create death pipe: %w", err)
	}
	urlR, urlW, err := os.Pipe()
	if err != nil {
		_ = deathR.Close()
		_ = deathW.Close()
		return "", nil, nil, fmt.Errorf("browser: failed to create url pipe: %w", err)
	}

	spec := supervisorSpec{ChromePath: chromePath, Args: args, Env: append(os.Environ(), extraEnv...)}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		_ = deathR.Close()
		_ = deathW.Close()
		_ = urlR.Close()
		_ = urlW.Close()
		return "", nil, nil, err
	}

	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}

	cmd := exec.Command(self)
	cmd.Env = append(os.Environ(),
		supervisorEnvFlag+"=1",
		supervisorEnvSpec+"="+string(specJSON),
	)
	cmd.ExtraFiles = []*os.File{deathR, urlW} // fd 3, fd 4 in the child, per runSupervisor
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = deathR.Close()
		_ = deathW.Close()
		_ = urlR.Close()
		_ = urlW.Close()
		return "", nil, nil, fmt.Errorf("browser: failed to start supervisor: %w", err)
	}

	// The child inherited these; the parent's own copies would otherwise
	// keep the pipes' read/write ends alive even after the child exits.
	_ = deathR.Close()
	_ = urlW.Close()

	type result struct {
		url string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		r := bufio.NewReader(urlR)
		line, err := r.ReadString('\n')
		ch <- result{strings.TrimSpace(line), err}
	}()

	select {
	case res := <-ch:
		_ = urlR.Close()
		if res.err != nil || res.url == "" {
			_ = cmd.Process.Kill()
			_ = deathW.Close()
			return "", nil, nil, fmt.Errorf("browser: supervisor did not report a DevTools URL: %v", res.err)
		}
		return res.url, cmd, deathW, nil
	case <-time.After(launchTimeout):
		_ = urlR.Close()
		_ = cmd.Process.Kill()
		_ = deathW.Close()
		return "", nil, nil, fmt.Errorf("browser: timed out waiting for supervised chrome to start")
	case <-ctx.Done():
		_ = urlR.Close()
		_ = cmd.Process.Kill()
		_ = deathW.Close()
		return "", nil, nil, ctx.Err()
	}
}

// findChromeExecPath mirrors chromedp's own (unexported) binary discovery
// so supervised mode — which bypasses chromedp.ExecAllocator entirely —
// finds Chrome the same way the non-supervised path does.
func findChromeExecPath() string {
	var locations []string
	switch runtime.GOOS {
	case "darwin":
		locations = []string{
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		}
	case "windows":
		locations = []string{"chrome", "chrome.exe"}
	default:
		locations = []string{
			"headless_shell", "headless-shell", "chromium", "chromium-browser",
			"google-chrome", "google-chrome-stable", "google-chrome-beta", "google-chrome-unstable",
			"/usr/bin/google-chrome", "/usr/local/bin/chrome", "/snap/bin/chromium", "chrome",
		}
	}
	for _, path := range locations {
		if found, err := exec.LookPath(path); err == nil {
			return found
		}
	}
	return "google-chrome"
}

// buildSupervisedArgs constructs Chrome's raw command-line arguments for
// supervised mode.
//
// This intentionally duplicates the flag lists in stealthExecOptions /
// chromedp.DefaultExecAllocatorOptions rather than reusing them:
// chromedp.ExecAllocatorOption values mutate a private *chromedp.
// ExecAllocator struct with no way to read the flags back out afterwards,
// and supervised mode bypasses chromedp.ExecAllocator entirely (Chrome is
// launched by the supervisor process, not by chromedp, so it can be
// Chrome's OS-level parent instead of us). Keep this in sync with
// stealthExecOptions if that changes.
func buildSupervisedArgs(cfg PoolConfig, userDataDir string) []string {
	var args []string
	if cfg.Stealth != StealthOff {
		args = append(args,
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-blink-features=AutomationControlled",
			"--disable-background-networking",
			"--enable-features=NetworkService,NetworkServiceInProcess",
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-breakpad",
			"--disable-client-side-phishing-detection",
			"--disable-default-apps",
			"--disable-extensions",
			"--disable-features=site-per-process,Translate,BlinkGenPropertyTrees",
			"--disable-hang-monitor",
			"--disable-ipc-flooding-protection",
			"--disable-popup-blocking",
			"--disable-prompt-on-repost",
			"--disable-renderer-backgrounding",
			"--disable-sync",
			"--force-color-profile=srgb",
			"--metrics-recording-only",
			"--safebrowsing-disable-auto-update",
			"--password-store=basic",
			"--use-mock-keychain",
			"--window-size=1920,1080",
		)
		if cfg.Stealth != StealthHeadful {
			args = append(args, "--headless")
		}
	} else {
		args = append(args,
			"--no-first-run",
			"--no-default-browser-check",
			"--headless",
			"--disable-background-networking",
			"--enable-features=NetworkService,NetworkServiceInProcess",
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-breakpad",
			"--disable-client-side-phishing-detection",
			"--disable-default-apps",
			"--disable-extensions",
			"--disable-features=site-per-process,Translate,BlinkGenPropertyTrees",
			"--disable-hang-monitor",
			"--disable-ipc-flooding-protection",
			"--disable-popup-blocking",
			"--disable-prompt-on-repost",
			"--disable-renderer-backgrounding",
			"--disable-sync",
			"--force-color-profile=srgb",
			"--metrics-recording-only",
			"--safebrowsing-disable-auto-update",
			"--enable-automation",
			"--password-store=basic",
			"--use-mock-keychain",
			"--hide-scrollbars",
			"--mute-audio",
		)
	}

	args = append(args, "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage")

	for name, value := range cfg.ExtraFlags {
		switch v := value.(type) {
		case bool:
			if v {
				args = append(args, "--"+name)
			}
		default:
			args = append(args, fmt.Sprintf("--%s=%v", name, v))
		}
	}

	if cfg.Proxy != nil {
		args = append(args,
			"--proxy-server="+cfg.Proxy.Server,
			"--proxy-bypass-list=<-loopback>",
		)
	}

	args = append(args,
		"--user-data-dir="+userDataDir,
		"--remote-debugging-port=0",
		"about:blank",
	)
	return args
}
