package katsuragi

import (
	"fmt"

	"golang.org/x/net/html"
)

// GetTitle fetches the title of a webpage given its URL
func (f *Fetcher) GetTitle(url string) (string, error) {
	isValid := validateURL(url)
	if !isValid {
		return "", fmt.Errorf("GetTitle failed to validate URL: %v", url)
	}
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

// // traverseAndExtractTitle traverses the HTML node and extracts the title of the webpage
func traverseAndExtractTitle(n *html.Node) (string, bool) {
	if n.Type == html.ElementNode && n.Parent.Data == "head" && (n.Data == "meta" || n.Data == "title") {
		// <title>...</title> tag
		if n.Data == "title" {
			if n.FirstChild != nil {
				// If the <title> tag has a child node, return the data of the child node
				return n.FirstChild.Data, true
			}
		} else if n.Data == "meta" {
			// <meta name="title" content="website title"/> tag
			var name, property, content string
			// Check for "name" attribute and "content" attribute
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
			// If name="title" and "content" is not empty, return the content
			if (name == "title" || name == "twitter:title" || property == "og:title") && content != "" {
				return content, true
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
