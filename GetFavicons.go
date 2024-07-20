package katsuragi

import (
	"fmt"
	Url "net/url"
	"strings"

	"golang.org/x/net/html"
)

// GetFavicon fetches the favicon of a webpage given its URL
func (f *Fetcher) GetFavicons(url string) ([]string, error) {
    isValid := validateURL(url)
    if !isValid {
        return nil, fmt.Errorf("GetFavicon failed to validate URL: %v", url)
    }
    htmlDoc, err := retrieveHTML(url, f)
    if err != nil {
        return nil, err
    }
    favicons, found := traverseAndExtractFavicons(htmlDoc, url)
    if !found {
        return nil, fmt.Errorf("GetFavicon failed to find any favicon in HTML")
    }
    return favicons, nil
}

// Valid tags for favicons
// Most of them are commented out because they are require additional checks, for example og:image requires a check for neighboring og:image:width and og:image:height tags.
var validRel = map[string]bool{
    "icon":                  true,
    // "shortcut icon":         true,
    "apple-touch-icon":      true,
    // "fluid-icon":            true,
    // "mask-icon":             true,
    // "alternate icon":        true,
}
var validMeta = map[string]bool{
    //"twitter:image:src": true, //twitter:image:src is the Twitter Cards version of og:image
    //"og:image":          true, //og:image is the Open Graph protocol's version of twitter:image:src
}



// traverseAndExtractFavicons traverses the HTML node tree and extracts favicon URLs
func traverseAndExtractFavicons(n *html.Node, url string) ([]string, bool) {
    var favicons []string

    if n.Type == html.ElementNode && (n.Data == "link" || n.Data == "meta") && n.Parent.Data == "head" {
        attrMap := extractAttributes(n.Attr)
        if n.Data == "link" {
            if rel, found := attrMap["rel"]; found && validRel[rel] {
                if href, found := attrMap["href"]; found {
                    favicons = append(favicons, href)
                }
            }
        } else if n.Data == "meta" {
            if name, found := attrMap["name"]; found && validMeta[name] {
                if content, found := attrMap["content"]; found {
                    favicons = append(favicons, content)
                }
            } else if property, found := attrMap["property"]; found && validMeta[property] {
                if content, found := attrMap["content"]; found {
                    favicons = append(favicons, content)
                }
            }
        }
    }

    for c := n.FirstChild; c != nil; c = c.NextSibling {
        if childFavicons, found := traverseAndExtractFavicons(c, url); found {
            favicons = append(favicons, childFavicons...)
        }
    }
    if len(favicons) > 0 {
        // remove duplicates
        favicons = removeDuplicatesFromSlice(favicons)
        // check if any links are relative. If they are, add the scheme + subdomain + domain + top level domain, and then append favicon url
        // just appending the favicon url will not work because it will be relative to the current page
        for i, faviconURL := range favicons {
            if !strings.HasPrefix(faviconURL, "http") {
                uri, _ := Url.Parse(url)
                favicons[i] = uri.Scheme + "://" + uri.Host + faviconURL
            }
        }
        return favicons, true
    }
    return nil, false
}