package katsuragi

import (
	"context"
	"fmt"

	"golang.org/x/net/html"
)

func (f *Fetcher) GetTitle(ctx context.Context, url string) (string, error) {
	doc, err := retrieveHTML(ctx, url, f)
	if err != nil {
		return "", err
	}
	title, found := f.traverseAndExtractTitle(doc)
	if !found {
		return "", fmt.Errorf("GetTitle failed to find title in HTML")
	}
	return title, nil
}

// Valid tags (package defaults; overridable per-Fetcher via FetcherProps.TitleTags)
var validTitleTags = map[string]bool{
	"title": true,
}

// Valid meta tags (package defaults; overridable per-Fetcher via FetcherProps.TitleMeta)
var validTitleMeta = map[string]bool{
	"title":         true,
	"twitter:title": true,
	"og:title":      true,
}

// traverseAndExtractTitle traverses the HTML node tree and extracts the title of the webpage
func (f *Fetcher) traverseAndExtractTitle(n *html.Node) (string, bool) {
	if n.Type == html.ElementNode {
		if f.props.TitleTags[n.Data] && n.Parent != nil && n.Parent.Data == "head" {
			if n.FirstChild != nil {
				// If the <title> tag has a child node, return the data of the child node
				return n.FirstChild.Data, true
			}
		} else if n.Data == "meta" {
			attrMap := extractAttributes(n.Attr) // Extract attributes to map
			if name, found := attrMap["name"]; found && f.props.TitleMeta[name] {
				if content, found := attrMap["content"]; found && content != "" {
					return content, true
				}
			} else if property, found := attrMap["property"]; found && f.props.TitleMeta[property] {
				if content, found := attrMap["content"]; found && content != "" {
					return content, true
				}
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if title, found := f.traverseAndExtractTitle(c); found {
			return title, true
		}
	}

	return "", false
}
