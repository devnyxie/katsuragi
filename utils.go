package katsuragi

import (
	"fmt"
	"net/http"
	Url "net/url"

	"golang.org/x/net/html"
)

// --- Generic utils ---
func retrieveHTML(url string, f *Fetcher) (*html.Node, error) {
	f.mu.Lock() // Lock the mutex
	// return if found
	if f.cache.url == url {
		cachedHtml := f.cache.response
		return cachedHtml, nil
	} else {
		// reset the cache if the URL is different
		f.cache = LastCachedResponse{}
	}
	f.mu.Unlock() // Unlock the mutex

	// if not found, make a request to the URL
	httpResp, err := http.Get(url)
	if err != nil {
		return &html.Node{}, fmt.Errorf("retrieveHTML could not reach the URL: %v", err)
	}
	defer httpResp.Body.Close() // Ensure the response body is closed to prevent memory leaks after the function returns
	if httpResp.StatusCode != http.StatusOK {
		return &html.Node{}, fmt.Errorf("retrieveHTML failed to fetch URL. HTTP Status: %v", httpResp.Status)
	}
	doc, err := html.Parse(httpResp.Body)
	if err != nil {
		return &html.Node{}, fmt.Errorf("retrieveHTML failed to parse HTML: %v", err)
	}

	// Store the result in the cache
	f.mu.Lock() // Lock the mutex
	f.cache = LastCachedResponse{url, doc}
	f.mu.Unlock() // Unlock the mutex

	// return the result
	return doc, nil
}
func validateURL(url string) bool {
	_, err := Url.Parse(url)
	return err == nil
}