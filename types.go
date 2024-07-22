package katsuragi

import (
	"container/list"
	"net/http"
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

var defaultFetcherProps = FetcherProps{
    Timeout:       3000 * time.Millisecond,
    CacheCap: 10,
}

func NewFetcher(props *FetcherProps) *Fetcher {
    if props == nil {
        props = &defaultFetcherProps
    } else {
        // Set default values for unspecified fields
        if props.Timeout == 0 {
            props.Timeout = defaultFetcherProps.Timeout
        }
        if props.CacheCap == 0 {
            props.CacheCap = defaultFetcherProps.CacheCap
        }
    }

    return &Fetcher{
        cache:   make(map[string]*list.Element),
        lruList: list.New(),
        props:   *props,
    }
}

type GetLinksProps struct {
    Url      string
    Category string
}

type cacheEntry struct {
    url      string
    response *html.Node
    isError  bool
    err      error
}

// HTTP Client
type UserAgentTransport struct {
    UserAgent string
    Transport http.RoundTripper
}

func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    req.Header.Set("User-Agent", t.UserAgent)
    return t.Transport.RoundTrip(req)
}