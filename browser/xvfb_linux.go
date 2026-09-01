//go:build linux

package browser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// xvfbDisplay is a running Xvfb (virtual X11 framebuffer) process, used by
// StealthHeadful to give Chrome a real display to run headful in rather
// than passing --headless at all.
type xvfbDisplay struct {
	display  string // X11 display number, without the leading colon (e.g. "99")
	authPath string
	cmd      *exec.Cmd
}

// startXvfb launches a new Xvfb process and waits for it to report which
// display number it bound to.
func startXvfb() (*xvfbDisplay, error) {
	if _, err := exec.LookPath("Xvfb"); err != nil {
		return nil, fmt.Errorf("browser: Xvfb not found on PATH (required for StealthHeadful, Linux only): %w", err)
	}

	authFile, err := os.CreateTemp("", "katsuragi-xvfb-auth-")
	if err != nil {
		return nil, err
	}
	authPath := authFile.Name()
	authFile.Close()

	// Xvfb reports the display number it bound to on this fd, since we
	// ask for an auto-assigned display rather than a fixed one (avoids
	// collisions between concurrent Pool instances on the same host).
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		os.Remove(authPath)
		return nil, err
	}

	cmd := exec.Command("Xvfb", "-displayfd", "3", "-nolisten", "tcp", "-screen", "0", "1920x1080x24")
	cmd.ExtraFiles = []*os.File{pipeWriter}
	cmd.Env = append(os.Environ(), "XAUTHORITY="+authPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}

	if err := cmd.Start(); err != nil {
		pipeReader.Close()
		pipeWriter.Close()
		os.Remove(authPath)
		return nil, err
	}
	pipeWriter.Close()

	type result struct {
		display string
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := bufio.NewReader(pipeReader).ReadString('\n')
		ch <- result{strings.TrimSpace(line), err}
	}()

	var display string
	select {
	case res := <-ch:
		if res.err != nil {
			_ = cmd.Process.Kill()
			os.Remove(authPath)
			return nil, fmt.Errorf("browser: failed to read Xvfb display number: %w", res.err)
		}
		if _, err := strconv.Atoi(res.display); err != nil {
			_ = cmd.Process.Kill()
			os.Remove(authPath)
			return nil, fmt.Errorf("browser: Xvfb reported an invalid display number %q", res.display)
		}
		display = res.display
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		os.Remove(authPath)
		return nil, fmt.Errorf("browser: timed out waiting for Xvfb to start")
	}

	xauth := exec.Command("xauth", "generate", ":"+display, ".", "trusted")
	xauth.Env = append(os.Environ(), "XAUTHORITY="+authPath)
	if err := xauth.Run(); err != nil {
		_ = cmd.Process.Kill()
		os.Remove(authPath)
		return nil, fmt.Errorf("browser: xauth generate failed: %w", err)
	}

	return &xvfbDisplay{display: display, authPath: authPath, cmd: cmd}, nil
}

// displayEnv and authEnv are passed to chromedp.Env so the launched Chrome
// process targets this virtual display.
func (x *xvfbDisplay) displayEnv() string { return "DISPLAY=:" + x.display }
func (x *xvfbDisplay) authEnv() string    { return "XAUTHORITY=" + x.authPath }

// stop terminates the Xvfb process, SIGKILLing it if it doesn't exit
// within killGrace, and removes its temporary auth file. Safe to call on a
// nil receiver (StealthBasic/StealthOff instances have no xvfbDisplay).
func (x *xvfbDisplay) stop(killGrace time.Duration) {
	if x == nil || x.cmd == nil || x.cmd.Process == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		_, _ = x.cmd.Process.Wait()
		close(done)
	}()

	_ = x.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(killGrace):
		_ = x.cmd.Process.Kill()
	}

	_ = os.Remove(x.authPath)
}
