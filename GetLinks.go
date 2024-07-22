package katsuragi

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func (f *Fetcher) GetLinks(props GetLinksProps) ([]string, error) {

	// Set default category to "all"
	if props.Category == "" {
		props.Category = "all"
	}

    htmlres, err := retrieveHTML(props.Url, f)
    if err != nil {
        return nil, err
    }

    var links []string
    var traverse func(*html.Node)

	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			attrMap := extractAttributes(n.Attr)
			if href, found := attrMap["href"]; found {
				isValid := validateUrl(href)
				newhref := ensureAbsoluteURL(href, props.Url);
				if props.Category == "all" {
					
					if !contains(links, newhref) && isValid {
						links = append(links, newhref)
					}
				} else if props.Category == "internal" {
					isInternal := IsInternalURL(newhref, props.Url)
					if isInternal && isValid {
						if !contains(links, newhref) {
							links = append(links, newhref)
						}
					}
				} else if props.Category == "external" {
					isInternal := IsInternalURL(newhref, props.Url)
					if !isInternal && isValid {
						if !contains(links, newhref) {
							links = append(links, newhref)
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}

	}
    traverse(htmlres)
	if len(links) == 0 {
		return nil, fmt.Errorf("GetLinks failed to find any links in HTML")
	}
    return links, nil
}

func IsInternalURL(href, urlStr string) bool {
    // Parse the found link
    parsedBacklink, err := url.Parse(href)
    if err != nil {
        return false
    }
	// Parse the original URL
	parsedUrl, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	// lets check if urlStr exists in href
	if strings.Contains(href, urlStr) {
		return true
	}

    // Check if the URL is from the same domain
    return parsedBacklink.Host == parsedUrl.Host
}


func validateUrl(urlStr string) bool {
	_, err := url.ParseRequestURI(urlStr)
	fmt.Println(err)
	return err == nil
}
