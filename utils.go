package katsuragi

import (
	"fmt"
	"net/http"
	Url "net/url"

	"golang.org/x/net/html"
)

// --- Generic utils ---
func retrieveHTML(url string, f *Fetcher) (*html.Node, error) {
	// if the URL is not the same as the last URL in the cache, clear the cache
	// ---
	// ---
	// try to get the result from the cache
	f.mu.Lock() // Lock the mutex
	cachedHtml, found := f.cache[url]
	f.mu.Unlock() // Unlock the mutex
	// return if found
	if found {
		return cachedHtml, nil
	}
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
	f.cache[url] = doc
	f.mu.Unlock() // Unlock the mutex

	// return the result
	return doc, nil
}
func validateURL(url string) bool {
	_, err := Url.Parse(url)
	return err == nil
}