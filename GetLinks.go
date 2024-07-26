package katsuragi

import (
	"fmt"
	"net/url"

	"golang.org/x/net/html"
)

// GetLinks fetches links from the given URL based on the category ("all", "internal", "external")
func (f *Fetcher) GetLinks(props GetLinksProps) ([]string, error) {
	// Set default category to "all"
	if props.Category == "" {
		props.Category = "all"
	}

    doc, err := retrieveHTML(props.Url, f)
	if err != nil {
		return nil, err
	}

    var links []string

    baseUrl, _ := url.Parse(props.Url)
    // *The error is ignored because the URL has been already validated in retrieveHTML.

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
					// base domain
					baseUrlParts, _ := extractDomainParts(props.Url)
					// *The error is ignored because the URL has been already validated in retrieveHTML.
					// link domain
					resolvedUrlParts, err := extractDomainParts(resolvedUrl)
					if err != nil {
						return
					}
					baseUrlDomain := baseUrlParts.Root + "." + baseUrlParts.TLD
					resolvedUrlDomain := resolvedUrlParts.Root + "." + resolvedUrlParts.TLD


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
		return nil, fmt.Errorf("GetTitle failed to find any links in HTML")
	}

    return links, nil
}
