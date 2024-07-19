package katsuragi

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type FetcherProps struct {
    UserAgent     string
    Timeout       time.Duration //ms
    CacheCap int
}

type Fetcher struct {
    cache     map[string]*list.Element
    lruList   *list.List
    mu        sync.RWMutex
    props     FetcherProps
}

var defaultProps = FetcherProps{
    Timeout:       3000 * time.Millisecond,
    CacheCap: 10,
}

func NewFetcher(props *FetcherProps) *Fetcher {
    if props == nil {
        props = &defaultProps
    } else {
        // Set default values for unspecified fields
        if props.Timeout == 0 {
            props.Timeout = defaultProps.Timeout
        }
        if props.CacheCap == 0 {
            props.CacheCap = defaultProps.CacheCap
        }
        if props.UserAgent == "" {
            props.UserAgent = defaultProps.UserAgent
        }
    }

    return &Fetcher{
        cache:   make(map[string]*list.Element),
        lruList: list.New(),
        props:   *props,
    }
}

type cacheEntry struct {
    url      string
    response *html.Node
    isError  bool
    err      error
}

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
    fmt.Println("Clearing cache")
    f.mu.Lock()
    defer f.mu.Unlock()

    f.cache = make(map[string]*list.Element)
    f.lruList = list.New()
}