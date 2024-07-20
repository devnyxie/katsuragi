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
        if cachedErr != nil {
            return nil, cachedErr
        }
        return cachedValue, nil
    }

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

func removeDuplicatesFromSlice(s []string) []string {
    m := make(map[string]bool)
    for _, item := range s {
        m[item] = true
    }
    var result []string
    for item := range m {
        result = append(result, item)
    }
    return result
}

// extractAttributes returns a map of html attribute keys and values
func extractAttributes(attrs []html.Attribute) map[string]string {
    attrMap := make(map[string]string, len(attrs))
    for _, attr := range attrs {
        attrMap[attr.Key] = attr.Val
    }
    return attrMap
}