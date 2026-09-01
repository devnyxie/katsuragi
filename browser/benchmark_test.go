//go:build integration

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// BenchmarkPool_ConcurrentCaptures exercises a realistically sized pool
// under concurrent load, producing a throughput/latency baseline useful for
// tuning PoolConfig.Size / MaxTabsPerBrowser in a deployment.
func BenchmarkPool_ConcurrentCaptures(b *testing.B) {
	pool, err := NewPool(context.Background(), PoolConfig{Size: 2, MaxTabsPerBrowser: 4})
	if err != nil {
		b.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>Bench</title></head><body></body></html>")
	}))
	defer server.Close()

	b.ResetTimer()

	const concurrency = 8
	var wg sync.WaitGroup
	work := make(chan struct{}, b.N)
	for i := 0; i < b.N; i++ {
		work <- struct{}{}
	}
	close(work)

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range work {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				tab, err := pool.Acquire(ctx)
				cancel()
				if err != nil {
					b.Errorf("acquire failed: %v", err)
					return
				}
				if err := chromedp.Run(tab.Ctx(), chromedp.Navigate(server.URL)); err != nil {
					b.Errorf("navigate failed: %v", err)
				}
				tab.Release()
			}
		}()
	}
	wg.Wait()
}
