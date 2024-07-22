package katsuragi

import (
	"fmt"
	"testing"

	"golang.org/x/net/html"
)

// Cache with error
func TestRetrieveHTML_CacheWithError(t *testing.T) {
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    testURL := "http://example.com"
    expectedErr := fmt.Errorf("cached error")
    f.addToCache(testURL, nil, expectedErr) // Simulate adding an error to the cache

    _, err := retrieveHTML(testURL, f)
    if err == nil || err.Error() != expectedErr.Error() {
        t.Fatalf("Expected cached error, got %v", err)
    }
}

// Cache without error
func TestRetrieveHTML_CacheWithoutError(t *testing.T) {
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    testURL := "http://example.com"
    cachedHead := &html.Node{} // Simulate a cached head node
    f.addToCache(testURL, cachedHead, nil)

    result, err := retrieveHTML(testURL, f)
    if err != nil || result != cachedHead {
        t.Fatalf("Expected cached head node without error, got %v, %v", result, err)
    }
}

// Add to cache and get from cache
func TestAddToCacheAndGetFromCache(t *testing.T) {
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 2})
    defer f.ClearCache()

    // Simulate adding a response to the cache
    dummyNode := &html.Node{}
    f.addToCache("http://example.com", dummyNode, nil)

    // Attempt to retrieve the cached response
    response, found, err := f.GetFromCache("http://example.com")
    if err != nil {
        t.Errorf("Expected no error, got: %v", err)
    }
    if !found {
        t.Errorf("Expected to find the entry in cache, but didn't")
    }
    if response != dummyNode {
        t.Errorf("Cached response does not match the expected response")
    }
}

// Get from cache with non-existent entry
func TestCacheEviction(t *testing.T) {
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 1})
    defer f.ClearCache()

    // Add two entries to the cache, which should trigger eviction of the first one
    f.addToCache("http://example.com", &html.Node{}, nil)
    f.addToCache("http://example.org", &html.Node{}, nil)

    // Attempt to retrieve the first entry, which should have been evicted
    _, found, _ := f.GetFromCache("http://example.com")
    if found {
        t.Errorf("Expected the first entry to be evicted, but it was found")
    }
}

// Clear cache
func TestClearCache(t *testing.T) {
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 1})
    // Add an entry to the cache
    f.addToCache("http://example.com", &html.Node{}, nil)

    // Clear the cache
    f.ClearCache()

    // Attempt to retrieve the entry after clearing the cache
    _, found, _ := f.GetFromCache("http://example.com")
    if found {
        t.Errorf("Expected no entries in cache after clearing, but found one")
    }
}

// elevate existing entry to the front of the LRU list
func TestAddToCache_UpdateExistingEntry(t *testing.T) {
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 2})
    defer f.ClearCache()

    // Add an entry to the cache
    dummyNode := &html.Node{}
    f.addToCache("http://example.com", dummyNode, nil)

    // Update the entry in the cache
    updatedNode := &html.Node{}
    f.addToCache("http://example.com", updatedNode, nil)

    // Retrieve the updated entry
    response, found, err := f.GetFromCache("http://example.com")
    if err != nil {
        t.Errorf("Expected no error, got: %v", err)
    }
    if !found {
        t.Errorf("Expected to find the entry in cache, but didn't")
    }
    if response != updatedNode {
        t.Errorf("Cached response does not match the updated response")
    }
}

// error entry should be returned as an error
func TestAddToCache_ErrorEntry(t *testing.T) {
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 2})
    defer f.ClearCache()

    ErrURLFetchFailed := fmt.Errorf("URL fetch failed")

    // Add an error entry to the cache
    f.addToCache("http://example.com", nil, ErrURLFetchFailed)

    // Retrieve the error entry
    response, found, err := f.GetFromCache("http://example.com")
    if err != ErrURLFetchFailed {
        t.Errorf("Expected error entry, got: %v", err)
    }
    if !found {
        t.Errorf("Expected to find the entry in cache, but didn't")
    }
    if response != nil {
        t.Errorf("Expected nil response, got: %v", response)
    }
}
