//go:build linux

package browser

import (
	"os/exec"
	"syscall"
)

// setDeathSignal asks the OS to SIGKILL cmd if this process dies.
//
// This restores parity with chromedp's own default behavior, which we'd
// otherwise silently lose: chromedp only sets Pdeathsig itself when no
// custom ModifyCmdFunc is supplied (see its allocate_linux.go), and our
// Pool needs ModifyCmdFunc to capture the *exec.Cmd for its own kill-grace
// logic in teardown().
//
// Verified during development that this is best-effort, not a guarantee:
// killing a parent process that had launched Chrome via chromedp — with
// Pdeathsig set, through chromedp's own unmodified default mechanism, no
// less — still left the Chrome process orphaned in testing. Pdeathsig is a
// known-unreliable Go/Linux mechanism in general (see
// https://go.dev/issue/27505 — it's tied to the specific OS thread that
// made the syscall, which Go's runtime doesn't guarantee stays alive for
// the process's lifetime), and Chrome's own internal process handling may
// compound that further. It doesn't hurt to set it, and it may still help
// in some environments, but it should not be relied on as the mitigation
// for orphaned Chrome processes after an abrupt (SIGKILL/OOM) parent
// death — see the package doc / README for the operational guidance that
// actually addresses this (running under a process supervisor that reaps
// orphans, e.g. tini/dumb-init in a container).
func setDeathSignal(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
