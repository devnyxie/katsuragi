package katsuragi

import (
	"fmt"
	"net/http"
	Url "net/url"
	"time"

	"golang.org/x/net/html"
)

// --- Generic utils ---
func retrieveHTML(url string, f *Fetcher) (*html.Node, error) {
    cachedValue, found, cachedErr := f.GetFromCache(url)
    if found {
        fmt.Println("cached value found, url:", url)
        if cachedErr != nil {
            return nil, cachedErr
        }
        return cachedValue, nil
    }
    fmt.Println("cached value not found, url:", url)

    timeout := time.Duration(f.props.Timeout) * time.Millisecond
    client := http.Client{
        Timeout: timeout,
    }

    httpResp, err := client.Get(url)
    if err != nil {
        cacheErr := fmt.Errorf("retrieveHTML could not reach the URL: %v", err)
        f.addToCache(url, nil, cacheErr)
        return nil, cacheErr
    }
    defer httpResp.Body.Close()

    if httpResp.StatusCode != http.StatusOK {
        cacheErr := fmt.Errorf("retrieveHTML failed to fetch URL. HTTP Status: %v", httpResp.Status)
        f.addToCache(url, nil, cacheErr)
        return nil, cacheErr
    }

    doc, err := html.Parse(httpResp.Body)
    if err != nil {
        cacheErr := fmt.Errorf("retrieveHTML failed to parse HTML: %v", err)
        f.addToCache(url, nil, cacheErr)
        return nil, cacheErr
    }

    f.addToCache(url, doc, nil)
    return doc, nil
}

func validateURL(url string) bool {
	_, err := Url.Parse(url)
	return err == nil
}