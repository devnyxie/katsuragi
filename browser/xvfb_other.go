//go:build !linux

package browser

import (
	"fmt"
	"time"
)

// xvfbDisplay is unused on non-Linux platforms; StealthHeadful is
// Linux-only (matches Xvfb's own platform support).
type xvfbDisplay struct{}

func startXvfb() (*xvfbDisplay, error) {
	return nil, fmt.Errorf("browser: StealthHeadful is only supported on Linux")
}

func (x *xvfbDisplay) displayEnv() string { return "" }
func (x *xvfbDisplay) authEnv() string    { return "" }
func (x *xvfbDisplay) stop(time.Duration) {}
