package katsuragi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	Url "net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

// --- Generic utils ---

// retrieveHTML fetches and parses url, using the Fetcher's cache and
// coalescing concurrent requests for the same URL into a single fetch.
func retrieveHTML(ctx context.Context, url string, f *Fetcher) (*html.Node, error) {
	if cachedValue, found, cachedErr := f.GetFromCache(url); found {
		if cachedErr != nil {
			return nil, cachedErr
		}
		return cachedValue, nil
	}

	v, err, _ := f.sf.Do(url, func() (interface{}, error) {
		return fetchAndParse(ctx, url, f)
	})
	if err != nil {
		return nil, err
	}
	return v.(*html.Node), nil
}

// fetchAndParse performs the actual HTTP fetch (with retries), parses and
// sanitizes the resulting HTML, and populates the cache.
func fetchAndParse(ctx context.Context, url string, f *Fetcher) (*html.Node, error) {
	body, err := fetchBody(ctx, url, f)
	if err != nil {
		f.addToCache(url, nil, err)
		return nil, err
	}

	doc, err := html.Parse(bytes.NewReader(body))
	// html.Parse is forgiving of malformed markup (won't error on garbled
	// or empty input), but it does reject pathologically deep documents
	// (an open-element stack past a fixed node limit) as a resource-
	// exhaustion guard, returning a nil doc with a non-nil error in that
	// case - that has to be handled, not ignored, since cleanHtml (and
	// every extraction function) assumes a non-nil root.
	if err != nil {
		err = fmt.Errorf("failed to parse HTML: %w", err)
		f.addToCache(url, nil, err)
		return nil, err
	}

	// Remove script and style tags
	cleanHtml(doc)

	if f.props.CacheRawBytes {
		f.addRawToCache(url, body)
	} else {
		f.addToCache(url, doc, nil)
	}
	return doc, nil
}

// fetchBody performs the HTTP GET (retrying transient failures according to
// f.props.MaxRetries/RetryBackoff) and returns the validated response body.
func fetchBody(ctx context.Context, url string, f *Fetcher) ([]byte, error) {
	client := f.httpClient()

	var lastErr error
	attempts := f.props.MaxRetries + 1
	backoff := f.props.RetryBackoff

	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff * time.Duration(1<<uint(attempt-1))):
			}
		}

		body, retryable, err := doFetch(ctx, client, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

// doFetch makes a single HTTP GET attempt. The bool return indicates whether
// the error (if any) is transient and worth retrying.
func doFetch(ctx context.Context, client *http.Client, url string) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false, err
	}

	httpResp, err := client.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		err := fmt.Errorf("retrieveHTML failed to fetch URL. HTTP Status: %v", httpResp.Status)
		return nil, httpResp.StatusCode >= 500, err
	}

	// if the content type is not text/html, return an error
	contentType := httpResp.Header.Get("Content-Type")
	mediaType, _, parseErr := mime.ParseMediaType(contentType)
	if parseErr != nil || mediaType != "text/html" {
		return nil, false, fmt.Errorf("retrieveHTML failed to fetch URL. Content-Type: %v", contentType)
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, true, err
	}
	return body, false, nil
}

// httpClient builds an *http.Client for this Fetcher's configuration.
func (f *Fetcher) httpClient() *http.Client {
	timeout := time.Duration(f.props.Timeout) * time.Millisecond

	transport := f.props.Transport
	if transport == nil {
		transport = &http.Transport{}
	}
	if f.props.UserAgent != "" {
		transport = &UserAgentTransport{
			UserAgent: f.props.UserAgent,
			Transport: transport,
		}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// cleanHtml removes script and style tags from the HTML
func cleanHtml(htmlres *html.Node) {
	var clean func(*html.Node)
	clean = func(n *html.Node) {
		var prev *html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "style") {
				if prev != nil {
					prev.NextSibling = c.NextSibling
				} else {
					n.FirstChild = c.NextSibling
				}
				if c.NextSibling != nil {
					c.NextSibling.PrevSibling = prev
				}
			} else {
				prev = c
				clean(c)
			}
		}
	}
	clean(htmlres)
}

// extractAttributes returns a map of html attribute keys and values
func extractAttributes(attrs []html.Attribute) map[string]string {
	attrMap := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		attrMap[attr.Key] = attr.Val
	}
	return attrMap
}

// contains checks if a string is in a slice
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func ensureAbsoluteURL(href, baseURL string) string {
	// Handle data URLs
	if strings.HasPrefix(href, "data:") {
		return href
	}
	// Parse the base URL
	baseUri, err := Url.Parse(baseURL)
	if err != nil {
		return href
	}
	// Parse the href
	uri, err := Url.Parse(href)
	if err != nil {
		return href
	}
	// If the href is already absolute, return it
	if uri.IsAbs() {
		return href
	}
	// Resolve the relative URL against the base URL
	return baseUri.ResolveReference(uri).String()
}

func extractDomainParts(rawURL string) (*DomainParts, error) {
	dp := &DomainParts{}

	// Parse the URL
	parsedURL, err := Url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	// Get the host
	host := parsedURL.Hostname()

	// Use the publicsuffix package to get the eTLD+1 (effective TLD plus one level)
	domainPlusOne, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return nil, fmt.Errorf("invalid domain: %v", err)
	}

	// Extract TLD
	tld, _ := publicsuffix.PublicSuffix(host)
	dp.TLD = tld

	// Extract root domain
	dp.Root = strings.TrimSuffix(domainPlusOne, "."+dp.TLD)

	// Extract subdomain (if any)
	if host != domainPlusOne {
		dp.Subdomain = strings.TrimSuffix(host, "."+domainPlusOne)
	}

	return dp, nil
}
