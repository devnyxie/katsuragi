package katsuragi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// brandPage returns an HTML document whose title contains brand (so
// pageContainsBrand verifies it) and that also carries a favicon link (so
// the follow-up f.GetFavicons call succeeds against the same page).
func brandPage(brand string) string {
	return fmt.Sprintf(`<html><head><title>%s - Official Site</title><link rel="icon" href="/favicon.ico"></head><body>Welcome to %s</body></html>`, brand, brand)
}

// unrelatedPage returns an HTML document that does not mention brand -
// simulates a parked/squatted domain or an irrelevant search result.
const unrelatedPage = `<html><head><title>Domain For Sale</title></head><body>This domain is parked.</body></html>`

func countingServer(t *testing.T, body string) (*httptest.Server, *int32) {
	t.Helper()
	var count int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(body))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, &count
}

// fakeSearchProvider is a SearchProvider test double.
type fakeSearchProvider struct {
	mu      sync.Mutex
	calls   int32
	results []string
	err     error
	delay   time.Duration
}

func (s *fakeSearchProvider) Search(ctx context.Context, query string) ([]string, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.results, s.err
}

func (s *fakeSearchProvider) callCount() int32 {
	return atomic.LoadInt32(&s.calls)
}

func TestGetFaviconByBrand_SearchWinsWithoutWaitingForSlowGuess(t *testing.T) {
	// The guess target sleeps well past the search branch's response, but
	// GetFaviconByBrand must not wait for it once search has verified.
	slowGuess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(700 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(brandPage("allegro")))
	}))
	defer slowGuess.Close()

	searchServer, _ := countingServer(t, brandPage("allegro"))
	defer searchServer.Close()

	search := &fakeSearchProvider{results: []string{searchServer.URL}}

	f := NewFetcher(&FetcherProps{
		Timeout:        3000,
		CacheCap:       10,
		SearchProvider: search,
		GuessDomains: func(brand, hint string) []string {
			return []string{slowGuess.URL}
		},
	})
	defer f.ClearCache()

	start := time.Now()
	favicons, err := f.GetFaviconByBrand(context.Background(), GetFaviconByBrandProps{Brand: "allegro"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(favicons) == 0 {
		t.Fatalf("expected at least one favicon, got none")
	}
	if !strings.HasPrefix(favicons[0], searchServer.URL) {
		t.Fatalf("expected favicon from search's domain %s, got %s", searchServer.URL, favicons[0])
	}
	if elapsed >= 700*time.Millisecond {
		t.Fatalf("expected GetFaviconByBrand to return without waiting for the slow guess (700ms), took %s", elapsed)
	}
}

func TestGetFaviconByBrand_SearchOverridesLandedGuess(t *testing.T) {
	// Guess verifies quickly for "allegro", but search (slightly slower)
	// disagrees and must win.
	guessServer, _ := countingServer(t, brandPage("allegro"))
	defer guessServer.Close()

	searchServer, _ := countingServer(t, brandPage("allegro"))
	defer searchServer.Close()

	search := &fakeSearchProvider{
		results: []string{searchServer.URL},
		delay:   150 * time.Millisecond,
	}

	f := NewFetcher(&FetcherProps{
		Timeout:        3000,
		CacheCap:       10,
		SearchProvider: search,
		GuessDomains: func(brand, hint string) []string {
			return []string{guessServer.URL}
		},
	})
	defer f.ClearCache()

	favicons, err := f.GetFaviconByBrand(context.Background(), GetFaviconByBrandProps{Brand: "allegro"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(favicons) == 0 || !strings.HasPrefix(favicons[0], searchServer.URL) {
		t.Fatalf("expected favicon from search's domain %s, got %v", searchServer.URL, favicons)
	}
}

func TestGetFaviconByBrand_GuessFallbackWhenSearchFails(t *testing.T) {
	guessServer, _ := countingServer(t, brandPage("allegro"))
	defer guessServer.Close()

	search := &fakeSearchProvider{err: errors.New("search unavailable")}

	f := NewFetcher(&FetcherProps{
		Timeout:        3000,
		CacheCap:       10,
		SearchProvider: search,
		GuessDomains: func(brand, hint string) []string {
			return []string{guessServer.URL}
		},
	})
	defer f.ClearCache()

	favicons, err := f.GetFaviconByBrand(context.Background(), GetFaviconByBrandProps{Brand: "allegro"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(favicons) == 0 || !strings.HasPrefix(favicons[0], guessServer.URL) {
		t.Fatalf("expected favicon from guess's domain %s, got %v", guessServer.URL, favicons)
	}
}

func TestGetFaviconByBrand_BothFail(t *testing.T) {
	guessServer, _ := countingServer(t, unrelatedPage)
	defer guessServer.Close()

	search := &fakeSearchProvider{err: errors.New("search unavailable")}

	f := NewFetcher(&FetcherProps{
		Timeout:        3000,
		CacheCap:       10,
		SearchProvider: search,
		GuessDomains: func(brand, hint string) []string {
			return []string{guessServer.URL}
		},
	})
	defer f.ClearCache()

	_, err := f.GetFaviconByBrand(context.Background(), GetFaviconByBrandProps{Brand: "allegro"})
	if err == nil {
		t.Fatalf("expected an error, got none")
	}
}

func TestGetFaviconByBrand_HintControlsGuessCount(t *testing.T) {
	tests := []struct {
		name          string
		hint          string
		expectedCalls int
	}{
		{name: "no hint - one guess", hint: "", expectedCalls: 1},
		{name: "with hint - two guesses", hint: "pl", expectedCalls: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, count := countingServer(t, unrelatedPage)
			defer server.Close()

			search := &fakeSearchProvider{err: errors.New("search unavailable")}

			var gotHint string
			f := NewFetcher(&FetcherProps{
				Timeout:        3000,
				CacheCap:       10,
				SearchProvider: search,
				GuessDomains: func(brand, hint string) []string {
					gotHint = hint
					candidates := []string{server.URL}
					if hint != "" {
						candidates = append(candidates, server.URL+"/x") // distinct path, still served by same server
					}
					return candidates
				},
			})
			defer f.ClearCache()

			_, _ = f.GetFaviconByBrand(context.Background(), GetFaviconByBrandProps{Brand: "allegro", Hint: test.hint})

			if gotHint != test.hint {
				t.Fatalf("expected GuessDomains to receive hint %q, got %q", test.hint, gotHint)
			}
			if got := int(atomic.LoadInt32(count)); got != test.expectedCalls {
				t.Fatalf("expected %d request(s) to the guess server, got %d", test.expectedCalls, got)
			}
		})
	}
}

func TestGetFaviconByBrand_CacheHitSkipsBothBranchesAndPopulatesOnFirstResolve(t *testing.T) {
	guessServer, guessCount := countingServer(t, brandPage("allegro"))
	defer guessServer.Close()

	search := &fakeSearchProvider{err: errors.New("search unavailable")}

	f := NewFetcher(&FetcherProps{
		Timeout:        3000,
		CacheCap:       10,
		SearchProvider: search,
		GuessDomains: func(brand, hint string) []string {
			return []string{guessServer.URL}
		},
	})
	defer f.ClearCache()

	props := GetFaviconByBrandProps{Brand: "allegro"}

	if _, err := f.GetFaviconByBrand(context.Background(), props); err != nil {
		t.Fatalf("first call: expected no error, got: %v", err)
	}
	firstGuessCalls := atomic.LoadInt32(guessCount)
	firstSearchCalls := search.callCount()
	if firstGuessCalls == 0 || firstSearchCalls == 0 {
		t.Fatalf("expected both branches to be exercised on first resolution, guess=%d search=%d", firstGuessCalls, firstSearchCalls)
	}

	if _, err := f.GetFaviconByBrand(context.Background(), props); err != nil {
		t.Fatalf("second call: expected no error, got: %v", err)
	}
	if got := atomic.LoadInt32(guessCount); got != firstGuessCalls {
		t.Fatalf("expected no additional guess requests on cache hit, before=%d after=%d", firstGuessCalls, got)
	}
	if got := search.callCount(); got != firstSearchCalls {
		t.Fatalf("expected no additional search calls on cache hit, before=%d after=%d", firstSearchCalls, got)
	}
}

func TestGetFaviconByBrand_EmptyBrand(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()

	_, err := f.GetFaviconByBrand(context.Background(), GetFaviconByBrandProps{Brand: "   "})
	expectedErr := "GetFaviconByBrand: brand must not be empty"
	if err == nil || err.Error() != expectedErr {
		t.Fatalf("expected error %q, got: %v", expectedErr, err)
	}
}

func TestGetFaviconByBrand_ContextCancellation(t *testing.T) {
	blockCh := make(chan struct{})
	guessServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blockCh
	}))
	defer guessServer.Close()
	defer close(blockCh)

	search := &fakeSearchProvider{delay: 5 * time.Second}

	f := NewFetcher(&FetcherProps{
		Timeout:            3000,
		CacheCap:           10,
		SearchProvider:     search,
		BrandGuessTimeout:  5 * time.Second,
		BrandSearchTimeout: 5 * time.Second,
		GuessDomains: func(brand, hint string) []string {
			return []string{guessServer.URL}
		},
	})
	defer f.ClearCache()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := f.GetFaviconByBrand(ctx, GetFaviconByBrandProps{Brand: "allegro"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected an error from context cancellation, got none")
	}
	if elapsed >= 1*time.Second {
		t.Fatalf("expected cancellation to propagate promptly (~100ms), took %s", elapsed)
	}
}
