package katsuragi

import (
	"context"
	"fmt"
	"net/http"
	Url "net/url"
	"strings"

	"golang.org/x/net/html"
)

// DuckDuckGoSearchProvider is the default SearchProvider: a dependency-free
// scraper of DuckDuckGo's no-JS HTML results endpoint. No API key required.
//
// This scrapes an undocumented HTML endpoint rather than an official API, so
// it is inherently fragile to markup changes and to rate-limiting/blocking
// based on User-Agent or request volume. BaseURL/HTTPClient are overridable
// (mirroring captcha.CapSolver's BaseURL/HTTPClient pattern) so callers who
// need something more robust can swap in their own SearchProvider (a paid
// search API, a curated brand->domain table, ...) via
// FetcherProps.SearchProvider without touching GetFaviconByBrand.
//
// The zero value is ready to use.
type DuckDuckGoSearchProvider struct {
	// BaseURL defaults to https://html.duckduckgo.com/html/; overridable
	// so tests can point it at a local mock server.
	BaseURL string
	// HTTPClient defaults to http.DefaultClient.
	HTTPClient *http.Client
	// MaxResults caps how many candidate result URLs Search returns,
	// best-first. Defaults to 5.
	MaxResults int
}

const defaultDuckDuckGoBaseURL = "https://html.duckduckgo.com/html/"
const defaultDuckDuckGoMaxResults = 5

func (d *DuckDuckGoSearchProvider) baseURL() string {
	if d.BaseURL != "" {
		return d.BaseURL
	}
	return defaultDuckDuckGoBaseURL
}

func (d *DuckDuckGoSearchProvider) httpClient() *http.Client {
	if d.HTTPClient != nil {
		return d.HTTPClient
	}
	return http.DefaultClient
}

func (d *DuckDuckGoSearchProvider) maxResults() int {
	if d.MaxResults > 0 {
		return d.MaxResults
	}
	return defaultDuckDuckGoMaxResults
}

var _ SearchProvider = (*DuckDuckGoSearchProvider)(nil)

// Search implements SearchProvider by scraping DuckDuckGo's HTML results
// page for the query.
func (d *DuckDuckGoSearchProvider) Search(ctx context.Context, query string) ([]string, error) {
	reqURL := d.baseURL() + "?q=" + Url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo search: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; katsuragi-search/1.0)")

	resp, err := d.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo search: unexpected status %s", resp.Status)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo search: %w", err)
	}

	var results []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			attrMap := extractAttributes(n.Attr)
			if strings.Contains(attrMap["class"], "result__a") {
				if href := resolveDuckDuckGoHref(attrMap["href"]); href != "" && !contains(results, href) {
					results = append(results, href)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(results) > d.maxResults() {
		results = results[:d.maxResults()]
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("duckduckgo search: no results for %q", query)
	}
	return results, nil
}

// resolveDuckDuckGoHref unwraps DuckDuckGo's HTML-endpoint redirect links
// (a scheme-relative "//duckduckgo.com/l/?uddg=<url-encoded target>&rut=...")
// into the real target URL. Returns href unchanged (made scheme-absolute if
// needed) if it isn't in that shape, and "" if href can't be parsed at all.
func resolveDuckDuckGoHref(href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}

	parsed, err := Url.Parse(href)
	if err != nil {
		return ""
	}
	if strings.HasSuffix(parsed.Hostname(), "duckduckgo.com") && strings.HasPrefix(parsed.Path, "/l/") {
		if target := parsed.Query().Get("uddg"); target != "" {
			return target
		}
	}
	return href
}
