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

// traverseAndExtractDescription traverses the HTML node and extracts the description of the webpage
func traverseAndExtractDescription(n *html.Node) (string, bool) {
	if n.Type == html.ElementNode && n.Data == "meta" {
		var name, property, content string
		for _, attr := range n.Attr {
			switch attr.Key {
			case "name":
				name = attr.Val
			case "property":
				property = attr.Val
			case "content":
				content = attr.Val
			}
		}
		if (name == "description" || name == "twitter:description" || property == "og:description" ) && content != "" {
			return content, true
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if description, found := traverseAndExtractDescription(c); found {
			return description, true
		}
	}

	return "", false
}
