package katsuragi

import (
	"container/list"

	"golang.org/x/net/html"
)

func (f *Fetcher) GetFromCache(url string) (*html.Node, bool, error) {
    f.mu.RLock()
    defer f.mu.RUnlock()

    if elem, ok := f.cache[url]; ok {
        f.lruList.MoveToFront(elem)
        entry := elem.Value.(*cacheEntry)
        if entry.isError {
            return nil, true, entry.err
        }
        return entry.response, true, nil
    }
    return nil, false, nil
}

func (f *Fetcher) addToCache(url string, response *html.Node, err error) {
    f.mu.Lock()
    defer f.mu.Unlock()

    isError := err != nil

    if elem, ok := f.cache[url]; ok {
        f.lruList.MoveToFront(elem)
        entry := elem.Value.(*cacheEntry)
        entry.response = response
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

    entry := &cacheEntry{url: url, response: response, isError: isError, err: err}
    elem := f.lruList.PushFront(entry)
    f.cache[url] = elem
}

func (f *Fetcher) ClearCache() {
    f.mu.Lock()
    defer f.mu.Unlock()

    f.cache = make(map[string]*list.Element)
    f.lruList = list.New()
}