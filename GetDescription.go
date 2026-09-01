package katsuragi

import (
	"context"
	"fmt"

	"golang.org/x/net/html"
)

func (f *Fetcher) GetDescription(ctx context.Context, url string) (string, error) {
	doc, err := retrieveHTML(ctx, url, f)
	if err != nil {
		return "", err
	}
	description, found := f.traverseAndExtractDescription(doc)
	if !found {
		return "", fmt.Errorf("GetDescription failed to find description in HTML")
	}
	return description, nil
}

// Predefined sets of valid `name` and `property` values (package defaults;
// overridable per-Fetcher via FetcherProps.DescriptionMeta)
var validDescriptionMeta = map[string]bool{
	"description":         true,
	"twitter:description": true,
	"og:description":      true,
}

// traverseAndExtractDescription traverses the HTML node tree and extracts description content
func (f *Fetcher) traverseAndExtractDescription(n *html.Node) (string, bool) {
	if n.Type == html.ElementNode {
		if n.Data == "meta" {
			attrMap := extractAttributes(n.Attr) // Extract attributes to map
			if name, found := attrMap["name"]; found && f.props.DescriptionMeta[name] {
				if content, found := attrMap["content"]; found && content != "" {
					return content, true
				}
			} else if property, found := attrMap["property"]; found && f.props.DescriptionMeta[property] {
				if content, found := attrMap["content"]; found && content != "" {
					return content, true
				}
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if description, found := f.traverseAndExtractDescription(c); found {
			return description, true
		}
	}

	return "", false
}
