package katsuragi

import (
	"context"
	"fmt"
	"net/url"

	"golang.org/x/net/html"
)

// GetLinks fetches links from the given URL based on the category ("all", "internal", "external")
func (f *Fetcher) GetLinks(ctx context.Context, props GetLinksProps) ([]string, error) {
	// Set default category to "all"
	if props.Category == "" {
		props.Category = "all"
	}

	doc, err := retrieveHTML(ctx, props.Url, f)
	if err != nil {
		return nil, err
	}

	var links []string

	baseUrl, _ := url.Parse(props.Url)
	// *The error is ignored because the URL has been already validated in retrieveHTML.

	// domainOf reduces a URL to its "Root.TLD" string, or "" if it can't be
	// determined - which happens for perfectly well-formed URLs too, e.g.
	// an IP-literal or "localhost" host has no eTLD+1. "" is a value the
	// switch below already has dedicated handling for (treated as "no
	// known domain to compare against"), so a failure here degrades
	// gracefully into that instead of dropping the link or panicking on a
	// nil result.
	domainOf := func(u string) string {
		parts, err := extractDomainParts(u)
		if err != nil {
			return ""
		}
		return parts.Root + "." + parts.TLD
	}
	baseUrlDomain := domainOf(props.Url)

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					// will be tested using bad links in html
					href, err := url.Parse(a.Val)
					if err != nil {
						continue
					}
					// absolute href (if relative, returns the same if not)
					resolvedUrl := baseUrl.ResolveReference(href).String()
					resolvedUrlDomain := domainOf(resolvedUrl)

					// Url.host will be different in cases like "http://example.com" and "http://www.example.com",
					// so we need to compare the domains instead.

					switch props.Category {
					case "all":
						links = append(links, resolvedUrl)
					case "internal":
						if resolvedUrlDomain == "" || resolvedUrlDomain == baseUrlDomain {
							links = append(links, resolvedUrl)
						}
					case "external":
						if resolvedUrlDomain != "" && resolvedUrlDomain != baseUrlDomain {
							links = append(links, resolvedUrl)
						}
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	if len(links) == 0 {
		return nil, fmt.Errorf("GetLinks failed to find any links in HTML")
	}

	return links, nil
}
