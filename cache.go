package katsuragi

import (
	"bytes"
	"container/list"

	"golang.org/x/net/html"
)

// GetFromCache returns the cached parsed document for url, if present.
//
// Note: this takes the same exclusive lock as writes, not a read lock, because
// a cache hit calls lruList.MoveToFront, which mutates the shared LRU list.
// Guarding that mutation with a read lock would let two concurrent readers
// mutate the list's internal pointers at once.
func (f *Fetcher) GetFromCache(url string) (*html.Node, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	elem, ok := f.cache[url]
	if !ok {
		return nil, false, nil
	}
	f.lruList.MoveToFront(elem)
	entry := elem.Value.(*cacheEntry)
	if entry.isError {
		return nil, true, entry.err
	}
	if f.props.CacheRawBytes {
		doc, _ := html.Parse(bytes.NewReader(entry.rawBytes))
		cleanHtml(doc)
		return doc, true, nil
	}
	return entry.response, true, nil
}

// addToCache stores a parsed document (or an error) under url.
func (f *Fetcher) addToCache(url string, response *html.Node, err error) {
	f.store(url, response, nil, err)
}

// addRawToCache stores the raw response body under url; it is re-parsed on
// every subsequent cache hit (see GetFromCache).
func (f *Fetcher) addRawToCache(url string, rawBytes []byte) {
	f.store(url, nil, rawBytes, nil)
}

func (f *Fetcher) store(url string, response *html.Node, rawBytes []byte, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	isError := err != nil

	if elem, ok := f.cache[url]; ok {
		f.lruList.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.response = response
		entry.rawBytes = rawBytes
		entry.isError = isError
		entry.err = err
		return
	}

	// Evict the least recently used entry if the cache is full
	if len(f.cache) >= f.props.CacheCap {
		oldest := f.lruList.Back()
		if oldest != nil {
			delete(f.cache, oldest.Value.(*cacheEntry).url)
			f.lruList.Remove(oldest)
		}
	}

	entry := &cacheEntry{url: url, response: response, rawBytes: rawBytes, isError: isError, err: err}
	elem := f.lruList.PushFront(entry)
	f.cache[url] = elem
}

func (f *Fetcher) ClearCache() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cache = make(map[string]*list.Element)
	f.lruList = list.New()
}
