package katsuragi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// TestFetcher_ConcurrentAccess exercises the cache + singleflight coalescing
// path under concurrent load. Run with -race to catch data races in the LRU
// cache bookkeeping (see cache.go's GetFromCache/store locking).
func TestFetcher_ConcurrentAccess(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Concurrent</title></head><body></body></html>`))
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})

	const goroutines = 50
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	titles := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			title, err := f.GetTitle(context.Background(), server.URL)
			if err != nil {
				errs <- err
				return
			}
			titles <- title
		}()
	}
	wg.Wait()
	close(errs)
	close(titles)

	for err := range errs {
		t.Errorf("unexpected error from concurrent GetTitle: %v", err)
	}
	for title := range titles {
		if title != "Concurrent" {
			t.Errorf("expected title %q, got %q", "Concurrent", title)
		}
	}
}

// TestFetcher_ConcurrentMixedURLs exercises the cache/LRU eviction path under
// concurrent access to more distinct URLs than the cache capacity, run with
// -race.
func TestFetcher_ConcurrentMixedURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Page</title></head><body></body></html>`))
	}))
	defer server.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 3})

	const goroutines = 30
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			url := server.URL + "/" + string(rune('a'+i%5))
			_, _ = f.GetTitle(context.Background(), url)
		}(i)
	}
	wg.Wait()
}
