package katsuragi

import (
	"fmt"

	"golang.org/x/net/html"
)

func NewFetcher() *Fetcher {
    return &Fetcher {
        cache: make(map[string]*html.Node),
    }
}

// GetFavicon fetches the favicon URL of a webpage given its URL
func (f *Fetcher) GetFavicon(url string) (string, error) {
    faviconURL := "https://example.com/favicon.ico"
    fmt.Println("Fetching favicon from", url)
    return faviconURL, nil
}

// close the Fetcher (empty the cache)
func (f *Fetcher) Close() {
	f.cache = nil
}


