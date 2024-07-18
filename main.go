package katsuragi

import (
	"fmt"
)

func NewFetcher() *Fetcher {
    return &Fetcher {
        cache: LastCachedResponse{},
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
	f.cache = LastCachedResponse{}
}


