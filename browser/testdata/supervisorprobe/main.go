// Command supervisorprobe is a throwaway harness for manually verifying
// PoolConfig.Supervised's orphan-prevention end to end: it launches a
// supervised pool, prints its own PID once Chrome is up, then blocks
// forever so an external script can SIGKILL it and check whether Chrome
// (and the supervisor) survive or die with it.
//
// Lives under testdata/ so `go build ./...`, `go vet ./...`, and
// golangci-lint (which follow the same testdata convention) all skip it.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/devnyxie/katsuragi/browser"
)

func main() {
	_, err := browser.NewPool(context.Background(), browser.PoolConfig{
		Size:       1,
		Supervised: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch failed:", err)
		os.Exit(1)
	}
	fmt.Println("READY", os.Getpid())
	select {}
}
