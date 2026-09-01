package katsuragi

import (
	"container/list"
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/sync/singleflight"
)

type FetcherProps struct {
	UserAgent string
	Timeout   time.Duration //ms
	CacheCap  int

	// CacheRawBytes, when true, caches the raw HTTP response body instead of
	// the parsed *html.Node tree. Cache hits are re-parsed on every read,
	// trading CPU for lower/more predictable memory usage and avoiding
	// sharing of parsed node trees across callers. Defaults to false.
	CacheRawBytes bool

	// MaxRetries is the number of additional attempts made after a transient
	// failure (network error or 5xx response) before giving up. Defaults to 0.
	MaxRetries int
	// RetryBackoff is the base delay between retry attempts (doubled after
	// each attempt). Defaults to 200ms when MaxRetries > 0.
	RetryBackoff time.Duration

	// Transport, if set, is used as the underlying http.RoundTripper instead
	// of a plain http.Transport. This is the extension point for callers who
	// want to layer in proxy selection, rate limiting, or other policy
	// without katsuragi needing to know about it.
	Transport http.RoundTripper

	// The following maps override the built-in sets of HTML tags/attributes
	// considered valid for each extraction feature. Leave nil to use the
	// package defaults.
	TitleTags       map[string]bool
	TitleMeta       map[string]bool
	DescriptionMeta map[string]bool
	FaviconRel      map[string]bool
	FaviconMeta     map[string]bool

	// SearchProvider is used by GetFaviconByBrand's search branch to
	// resolve a brand name to candidate domains. Defaults to a
	// dependency-free DuckDuckGo HTML-results scraper (duckduckgo.go)
	// when nil.
	SearchProvider SearchProvider

	// BrandCache caches brand(+hint) -> resolved-domain lookups made by
	// GetFaviconByBrand. Deliberately separate from the *html.Node cache
	// above, which is keyed by URL and shaped for parsed pages, not
	// domain strings. Defaults to a process-local in-memory LRU+TTL
	// (InMemoryBrandCache) when nil; supply your own (Redis, Memcached,
	// ...) for multi-process deployments.
	BrandCache BrandCache

	// BrandGuessTimeout and BrandSearchTimeout bound GetFaviconByBrand's
	// two concurrent resolution branches so a slow one can't block
	// indefinitely. Default to 2s / 4s respectively when zero.
	BrandGuessTimeout  time.Duration
	BrandSearchTimeout time.Duration

	// GuessDomains returns the candidate domain URLs GetFaviconByBrand's
	// guess branch verifies, in priority order, given an already-
	// sanitized brand (lowercase, [a-z0-9-] only) and hint. Defaults to
	// ["https://<brand>.com"] (+ ["https://<brand>.<hint>"] if hint !=
	// ""). Overridable so tests can point guesses at a mock server, and
	// so callers can customize the guess strategy (extra TLDs, an
	// allow-list, ...).
	GuessDomains func(brand, hint string) []string
}

type Fetcher struct {
	cache   map[string]*list.Element
	lruList *list.List
	mu      sync.Mutex
	props   FetcherProps
	sf      singleflight.Group
}

var defaultFetcherProps = FetcherProps{
	Timeout:  3000 * time.Millisecond,
	CacheCap: 10,
}

const defaultRetryBackoff = 200 * time.Millisecond
const defaultBrandGuessTimeout = 2 * time.Second
const defaultBrandSearchTimeout = 4 * time.Second

func NewFetcher(props *FetcherProps) *Fetcher {
	p := defaultFetcherProps
	if props != nil {
		p = *props
		// Set default values for unspecified fields
		if p.Timeout == 0 {
			p.Timeout = defaultFetcherProps.Timeout
		}
		if p.CacheCap == 0 {
			p.CacheCap = defaultFetcherProps.CacheCap
		}
	}
	if p.MaxRetries > 0 && p.RetryBackoff == 0 {
		p.RetryBackoff = defaultRetryBackoff
	}
	if p.TitleTags == nil {
		p.TitleTags = validTitleTags
	}
	if p.TitleMeta == nil {
		p.TitleMeta = validTitleMeta
	}
	if p.DescriptionMeta == nil {
		p.DescriptionMeta = validDescriptionMeta
	}
	if p.FaviconRel == nil {
		p.FaviconRel = validRel
	}
	if p.FaviconMeta == nil {
		p.FaviconMeta = validMeta
	}
	if p.SearchProvider == nil {
		p.SearchProvider = &DuckDuckGoSearchProvider{}
	}
	if p.BrandCache == nil {
		p.BrandCache = &InMemoryBrandCache{}
	}
	if p.BrandGuessTimeout == 0 {
		p.BrandGuessTimeout = defaultBrandGuessTimeout
	}
	if p.BrandSearchTimeout == 0 {
		p.BrandSearchTimeout = defaultBrandSearchTimeout
	}
	if p.GuessDomains == nil {
		p.GuessDomains = defaultGuessDomains
	}

	return &Fetcher{
		cache:   make(map[string]*list.Element),
		lruList: list.New(),
		props:   p,
	}
}

// ContentFetcher is the public contract implemented by *Fetcher. It exists so
// consumers (and tests) can depend on an interface instead of the concrete
// type, and so other packages/services built around katsuragi have a stable
// seam to route static-HTML extraction work through.
type ContentFetcher interface {
	GetTitle(ctx context.Context, url string) (string, error)
	GetDescription(ctx context.Context, url string) (string, error)
	GetFavicons(ctx context.Context, url string) ([]string, error)
	GetLinks(ctx context.Context, props GetLinksProps) ([]string, error)
	GetFaviconByBrand(ctx context.Context, props GetFaviconByBrandProps) ([]string, error)
}

var _ ContentFetcher = (*Fetcher)(nil)

type GetLinksProps struct {
	Url      string
	Category string
}

// GetFaviconByBrandProps are the parameters for GetFaviconByBrand.
type GetFaviconByBrandProps struct {
	// Brand is the bare brand name to resolve, e.g. "allegro". Required.
	Brand string
	// Hint is an optional TLD/country hint (e.g. "pl") tried alongside
	// the default ".com" domain guess. Leave empty for no hint.
	Hint string
}

// SearchProvider resolves a free-text query to candidate result URLs,
// best-first. Used by GetFaviconByBrand's search branch; see
// DuckDuckGoSearchProvider (duckduckgo.go) for the default implementation.
type SearchProvider interface {
	Search(ctx context.Context, query string) ([]string, error)
}

type DomainParts struct {
	Subdomain string
	Root      string
	TLD       string
}

type cacheEntry struct {
	url      string
	response *html.Node // populated unless CacheRawBytes is set
	rawBytes []byte     // populated only when CacheRawBytes is set
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
