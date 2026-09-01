//go:build !linux

package browser

import "os/exec"

// setDeathSignal is a no-op on non-Linux platforms: Pdeathsig is a
// Linux-specific syscall.SysProcAttr field. On other platforms, a killed
// parent process can still leave Chrome running (the same caveat every
// exec.Cmd-based tool has there).
func setDeathSignal(cmd *exec.Cmd) {}
