//go:build integration && linux

package browser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestIntegration_Supervised_SurvivesParentSIGKILL verifies the core claim
// of PoolConfig.Supervised: an abrupt, unmaskable death of the process
// that owns the Pool (simulated here with SIGKILL, since that's the one
// signal no amount of application-level cleanup code can react to) does
// not orphan Chrome. It builds and runs the browser/testdata/
// supervisorprobe helper (a tiny program that starts a Supervised pool
// and blocks forever), SIGKILLs it once Chrome is confirmed up, and
// asserts the entire process tree underneath it - the supervisor and
// every Chrome process it spawned, not just the main one - is gone
// shortly after, reaped by the supervisor rather than left running under
// init.
func TestIntegration_Supervised_SurvivesParentSIGKILL(t *testing.T) {
	probeBin := buildSupervisorProbe(t)

	cmd := exec.Command(probeBin)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to attach stdout pipe: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start probe: %v", err)
	}
	probePID := cmd.Process.Pid
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	if err := waitForReady(stdout, 15*time.Second); err != nil {
		t.Fatalf("probe never reported ready: %v", err)
	}

	supervisorPID, err := soleChild(probePID)
	if err != nil {
		t.Fatalf("failed to find the supervisor process (child of probe %d): %v", probePID, err)
	}
	chromePID, err := soleChild(supervisorPID)
	if err != nil {
		t.Fatalf("failed to find the chrome process (child of supervisor %d): %v", supervisorPID, err)
	}

	tree := append([]int{supervisorPID, chromePID}, allDescendants(chromePID)...)
	if len(tree) < 2 {
		t.Fatalf("expected at least the supervisor and chrome to be tracked, got %v", tree)
	}
	t.Logf("tracking %d processes under the probe before kill: %v", len(tree), tree)

	if err := syscall.Kill(probePID, syscall.SIGKILL); err != nil {
		t.Fatalf("failed to SIGKILL probe %d: %v", probePID, err)
	}

	deadline := time.Now().Add(10 * time.Second)
	var alive []int
	for time.Now().Before(deadline) {
		alive = nil
		for _, pid := range tree {
			if processAlive(pid) {
				alive = append(alive, pid)
			}
		}
		if len(alive) == 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("expected the supervisor+chrome process tree to be fully reaped after the parent was SIGKILLed, still alive: %v", alive)
}

// buildSupervisorProbe compiles browser/testdata/supervisorprobe into
// t.TempDir() and returns its path. It lives under testdata/ specifically
// so `go build ./...`/`go vet ./...`/lint skip it (Go tooling's own
// testdata convention) while still being reachable here by explicit path.
func buildSupervisorProbe(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "supervisorprobe")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/supervisorprobe")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build supervisorprobe: %v\n%s", err, out)
	}
	return bin
}

// waitForReady scans r for the "READY <pid>" line supervisorprobe prints
// once its supervised pool has launched successfully.
func waitForReady(r io.Reader, timeout time.Duration) error {
	type result struct{ err error }
	ch := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "READY") {
				ch <- result{}
				return
			}
		}
		ch <- result{err: fmt.Errorf("probe exited without printing READY: %v", scanner.Err())}
	}()
	select {
	case res := <-ch:
		return res.err
	case <-time.After(timeout):
		return fmt.Errorf("timed out after %s waiting for READY", timeout)
	}
}

// soleChild returns pid's one direct child, failing if it has zero or
// more than one - used here because both the probe->supervisor and
// supervisor->chrome relationships are known to be exactly one process.
func soleChild(pid int) (int, error) {
	children, err := directChildren(pid)
	if err != nil {
		return 0, err
	}
	if len(children) != 1 {
		return 0, fmt.Errorf("expected exactly one child of pid %d, got %v", pid, children)
	}
	return children[0], nil
}

func directChildren(pid int) ([]int, error) {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil // pgrep exits 1 when it finds nothing
		}
		return nil, err
	}
	var pids []int
	for _, line := range strings.Fields(string(out)) {
		n, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, n)
	}
	return pids, nil
}

// allDescendants recursively collects every descendant of pid via pgrep
// -P, matching Chrome's own zygote/renderer/GPU-process tree shape.
func allDescendants(pid int) []int {
	var all []int
	children, err := directChildren(pid)
	if err != nil {
		return all
	}
	for _, c := range children {
		all = append(all, c)
		all = append(all, allDescendants(c)...)
	}
	return all
}

func processAlive(pid int) bool {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}
