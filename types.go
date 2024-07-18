package katsuragi

import (
	"sync"

	"golang.org/x/net/html"
)

//to-be-used
type FaviconOptions struct {
	Validate   bool
	MaxDepth   int
	ReturnType string
	Quality    string
	Binary     bool
}

type Fetcher struct {
	Concurrency	     bool
	UserAgent  	 	 string
    cache	 LastCachedResponse // cache:str x str object
    mu 			     sync.Mutex 	// mutex for concurrent access to cache
}


type LastCachedResponse struct {
	url string
	response *html.Node
}