package katsuragi

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

// GetFaviconByBrand resolves a bare brand name (e.g. "allegro") to a
// favicon, without the caller supplying a URL. Hint is an optional
// TLD/country hint (e.g. "pl") tried alongside the default ".com" guess.
//
// Domain resolution races two independent strategies concurrently: cheap
// ".com"/hint guessing, and a pluggable SearchProvider lookup (see
// FetcherProps.SearchProvider, defaulting to DuckDuckGoSearchProvider).
// Both are cross-checked against actual page content (title/body text
// contains the brand name) before being trusted, to reject parked/squatted
// domains and irrelevant search results. If neither branch produces a
// verified match, an error is returned - notably, a real site whose
// homepage is a JS-rendered SPA with no server-side brand text will fail
// this check, since this is a static-HTML fetch with no browser rendering.
//
// Resolved domains are cached in FetcherProps.BrandCache (default:
// process-local in-memory, TTL 24h) so repeat lookups for the same brand
// skip resolution entirely.
func (f *Fetcher) GetFaviconByBrand(ctx context.Context, props GetFaviconByBrandProps) ([]string, error) {
	brand := strings.ToLower(strings.TrimSpace(props.Brand))
	hint := strings.ToLower(strings.TrimSpace(props.Hint))
	if brand == "" {
		return nil, fmt.Errorf("GetFaviconByBrand: brand must not be empty")
	}
	// hint is folded into the cache key so allegro/"" and allegro/"pl"
	// don't collide.
	key := brand + "\x00" + hint

	if domain, ok := f.props.BrandCache.Get(key); ok {
		return f.GetFavicons(ctx, domain)
	}

	domain, err := f.resolveBrandDomain(ctx, brand, hint)
	if err != nil {
		return nil, fmt.Errorf("GetFaviconByBrand: %w", err)
	}

	f.props.BrandCache.Set(key, domain)
	return f.GetFavicons(ctx, domain)
}

// brandSearchVerifyLimit caps how many of the search branch's results get a
// verification fetch, bounding worst-case latency/requests per call.
const brandSearchVerifyLimit = 3

// resolveBrandDomain races the guess and search branches concurrently.
// The search branch is authoritative: the instant it delivers a verified
// result, it wins outright, even if the guess branch hasn't landed yet -
// this is what keeps the common case fast. A guess that lands first is
// held (not returned) until search also reports, since search can still
// override it; if search then fails/empties, the held guess is used as
// fallback. Each branch runs under its own timeout so a stuck branch can't
// block the race indefinitely.
func (f *Fetcher) resolveBrandDomain(ctx context.Context, brand, hint string) (string, error) {
	type branchResult struct {
		domain string
		err    error
	}

	guessCtx, cancelGuess := context.WithTimeout(ctx, f.props.BrandGuessTimeout)
	defer cancelGuess()
	searchCtx, cancelSearch := context.WithTimeout(ctx, f.props.BrandSearchTimeout)
	defer cancelSearch()

	guessCh := make(chan branchResult, 1)
	searchCh := make(chan branchResult, 1)

	go func() {
		d, err := f.guessBranch(guessCtx, brand, hint)
		guessCh <- branchResult{d, err}
	}()
	go func() {
		d, err := f.searchBranch(searchCtx, brand)
		searchCh <- branchResult{d, err}
	}()

	var guessDone, searchDone bool
	var guessRes, searchRes branchResult

	for !guessDone || !searchDone {
		select {
		case guessRes = <-guessCh:
			guessDone = true
		case searchRes = <-searchCh:
			searchDone = true
			if searchRes.err == nil && searchRes.domain != "" {
				return searchRes.domain, nil
			}
		}
	}

	if guessRes.err == nil && guessRes.domain != "" {
		return guessRes.domain, nil
	}
	return "", fmt.Errorf("could not resolve domain for brand %q (guess: %v, search: %v)", brand, guessRes.err, searchRes.err)
}

// guessBranch tries the domains from FetcherProps.GuessDomains (by default,
// brand.com and brand.<hint> if hint != "") and returns the first one (in
// that order) that verifies.
func (f *Fetcher) guessBranch(ctx context.Context, brand, hint string) (string, error) {
	sanitized := sanitizeBrandForDomain(brand)
	if sanitized == "" {
		return "", fmt.Errorf("brand %q not guessable as a domain", brand)
	}
	candidates := f.props.GuessDomains(sanitized, hint)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no domain guesses for brand %q", brand)
	}
	return f.verifyCandidates(ctx, brand, candidates)
}

// defaultGuessDomains is the default FetcherProps.GuessDomains: brand.com,
// plus brand.<hint> if hint is non-empty.
func defaultGuessDomains(brand, hint string) []string {
	candidates := []string{"https://" + brand + ".com"}
	if hint != "" {
		candidates = append(candidates, "https://"+brand+"."+hint)
	}
	return candidates
}

// searchBranch queries the configured SearchProvider and verifies its top
// results, returning the first (by search rank) that verifies.
func (f *Fetcher) searchBranch(ctx context.Context, brand string) (string, error) {
	results, err := f.props.SearchProvider.Search(ctx, brand)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("search returned no results for %q", brand)
	}
	if len(results) > brandSearchVerifyLimit {
		results = results[:brandSearchVerifyLimit]
	}
	return f.verifyCandidates(ctx, brand, results)
}

// verifyCandidates concurrently fetches each candidate URL and content-
// checks it for brand; returns the first candidate (by original rank
// order) that verifies. Powers both the guess and search branches.
func (f *Fetcher) verifyCandidates(ctx context.Context, brand string, candidates []string) (string, error) {
	verified := make([]bool, len(candidates))
	var wg sync.WaitGroup
	for i, candidate := range candidates {
		wg.Add(1)
		go func(i int, candidate string) {
			defer wg.Done()
			doc, err := retrieveHTML(ctx, candidate, f)
			if err != nil {
				return
			}
			verified[i] = f.pageContainsBrand(doc, brand)
		}(i, candidate)
	}
	wg.Wait()

	for i, ok := range verified {
		if ok {
			return candidates[i], nil
		}
	}
	return "", fmt.Errorf("no candidate verified for brand %q (tried %v)", brand, candidates)
}

// brandVerifyTextLimit bounds how much visible body text pageContainsBrand
// scans, keeping the check cheap.
const brandVerifyTextLimit = 2000

// pageContainsBrand reports whether doc's <title> (via the existing
// f.traverseAndExtractTitle, honoring FetcherProps.TitleTags/TitleMeta
// overrides) or a bounded snippet of its visible body text contains brand
// as a case-insensitive substring. Used to reject parked/squatted domains
// that return 200 but aren't the real site.
func (f *Fetcher) pageContainsBrand(doc *html.Node, brand string) bool {
	needle := strings.ToLower(brand)
	if title, ok := f.traverseAndExtractTitle(doc); ok && strings.Contains(strings.ToLower(title), needle) {
		return true
	}
	return strings.Contains(strings.ToLower(extractVisibleText(doc, brandVerifyTextLimit)), needle)
}

// extractVisibleText concatenates TextNode data under doc, stopping once
// limit bytes have been collected. script/style content is already
// stripped by cleanHtml (run in fetchAndParse) before this ever sees the
// tree.
func extractVisibleText(doc *html.Node, limit int) string {
	var b strings.Builder
	var walk func(*html.Node) bool
	walk = func(n *html.Node) bool {
		if b.Len() >= limit {
			return true
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if walk(c) {
				return true
			}
		}
		return false
	}
	walk(doc)
	s := b.String()
	if len(s) > limit {
		s = s[:limit]
	}
	return s
}

// sanitizeBrandForDomain lowercases brand and strips anything that isn't a
// valid domain-label character ([a-z0-9-]), trimming stray leading/trailing
// hyphens. Multi-word brands (e.g. "coca cola") collapse to a single label
// ("cocacola") - a reasonable degradation, since the search branch doesn't
// share this limitation and remains the primary resolution path for such
// brands.
func sanitizeBrandForDomain(brand string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(brand) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}
