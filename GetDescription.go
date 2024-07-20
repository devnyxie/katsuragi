package katsuragi

import (
	"fmt"

	"golang.org/x/net/html"
)


func (f *Fetcher) GetDescription(url string) (string, error) {
	isValid := validateURL(url)
	if !isValid {
		return "", fmt.Errorf("GetDescription failed to validate URL: %v", url)
	}
    html, err := retrieveHTML(url, f)
	if err != nil {
		return "", err
	}
	description, found := traverseAndExtractDescription(html)
	if !found {
		return "", fmt.Errorf("GetDescription failed to find description in HTML")
	}
    return description, nil
}

// Predefined sets of valid `name` and `property` values
var validDescriptionMeta = map[string]bool{
    "description":       true,
    "twitter:description": true,
    "og:description":    true,
}

// traverseAndExtractDescription traverses the HTML node tree and extracts description content
func traverseAndExtractDescription(n *html.Node) (string, bool) {
    if n.Type == html.ElementNode && n.Data == "meta" && n.Parent.Data == "head" {
        attrMap := extractAttributes(n.Attr) // Extract attributes to map
        if name, found := attrMap["name"]; found && validDescriptionMeta[name] {
            if content, found := attrMap["content"]; found && content != "" {
                return content, true
            }
        } else if property, found := attrMap["property"]; found && validDescriptionMeta[property] {
            if content, found := attrMap["content"]; found && content != "" {
                return content, true
            }
        }
    }

    for c := n.FirstChild; c != nil; c = c.NextSibling {
        if description, found := traverseAndExtractDescription(c); found {
            return description, true
        }
    }

    return "", false
}