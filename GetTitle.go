package katsuragi

import (
	"fmt"

	"golang.org/x/net/html"
)

func (f *Fetcher) GetTitle(url string) (string, error) {
    html, err := retrieveHTML(url, f)
	if err != nil {
		return "", err
	}
	title, found := traverseAndExtractTitle(html)
	if !found {
		return "", fmt.Errorf("GetTitle failed to find title in HTML")
	}
    return title, nil
}

// Valid tags
var validTitleTags = map[string]bool{
    "title": true,
}

// Valid meta tags
var validTitleMeta = map[string]bool{
    "title":          true,
    "twitter:title":  true,
    "og:title":       true,
}

// traverseAndExtractTitle traverses the HTML node tree and extracts the title of the webpage
func traverseAndExtractTitle(n *html.Node) (string, bool) {
    if n.Type == html.ElementNode {
        if  validTitleTags[n.Data] && n.Parent != nil && n.Parent.Data == "head" {
            if n.FirstChild != nil {
                // If the <title> tag has a child node, return the data of the child node
                return n.FirstChild.Data, true
            }
        } else if n.Data == "meta" {
            attrMap := extractAttributes(n.Attr) // Extract attributes to map
            if name, found := attrMap["name"]; found && validTitleMeta[name] {
                if content, found := attrMap["content"]; found && content != "" {
                    return content, true
                }
            } else if property, found := attrMap["property"]; found && validTitleMeta[property] {
                if content, found := attrMap["content"]; found && content != "" {
                    return content, true
                }
            }
        }
    }

    for c := n.FirstChild; c != nil; c = c.NextSibling {
        if title, found := traverseAndExtractTitle(c); found {
            return title, true
        }
    }

    return "", false
}
