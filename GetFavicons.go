package katsuragi

import (
	"fmt"
	"net/http"
	Url "net/url"

	"golang.org/x/net/html"
)

func (f *Fetcher) GetFavicons(url string) ([]string, error) {
    htmlDoc, err := retrieveHTML(url, f)
    if err != nil {
        return nil, err
    }
    favicons, found := traverseAndExtractFavicons(htmlDoc, url)
    if !found {
        getRootFaviconIco(&favicons, url)
    }
    // if not found, return error
    if !found && len(favicons) == 0 {
        return nil, fmt.Errorf("GetFavicon failed to find any favicons")
    }
    return favicons, nil
}

// valid tags for favicons
var validRel = map[string]bool{
    "icon":                  true,
    "apple-touch-icon":      true,
    "shortcut icon":         true,

    // to be reviewed:
    // "fluid-icon":            true,
    // "mask-icon":             true,
    // "alternate icon":        true,
}
var validMeta = map[string]bool{
    "og:image":          true,

    // to be reviewed:
    //"twitter:image:src": true, 
    //"twitter:image":     true, 
}


// Many websites, "https://docs.microsoft.com" for example, do not have a favicon tag in the HTML, but
// they have a favicon.ico file in the root directory which is fetched by browsers.
// This function tries to fetch the favicon.ico file from the root directory of the website. If the file is found,
// it is added to the list of favicons.
func getRootFaviconIco(existingFavicons *[]string, url string) error {
    parsedUrl, _ := Url.Parse(url)
    domain := parsedUrl.Scheme + "://" + parsedUrl.Host + "/favicon.ico"
    if !contains(*existingFavicons, domain) {
        // test: mockup server, 200 "/", 404 "/favicon.ico"
        resp, err := http.Get(domain)
        if err != nil {
            return fmt.Errorf("failed to fetch favicon.ico: %w", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode == http.StatusOK {
            *existingFavicons = append(*existingFavicons, domain)
        } else {
            fmt.Println("Favicon.ico not found:", resp.Status)
        }
    }
    return nil
}

// traverseAndExtractFavicons traverses the HTML node tree and extracts favicon URLs
func traverseAndExtractFavicons(n *html.Node, url string) ([]string, bool) {
    var favicons []string

    if n.Type == html.ElementNode && (n.Data == "link" || n.Data == "meta") && n.Parent.Data == "head" {
        attrMap := extractAttributes(n.Attr)
        if n.Data == "link" {
            if rel, found := attrMap["rel"]; found && validRel[rel] {
                if href, found := attrMap["href"]; found {
                    if !contains(favicons, href) {
                        favicons = append(favicons, href)
                    }

                }
            }
        // og:image + aspect ratio check
        } else if n.Data == "meta" {
            if property, found := attrMap["property"]; found && validMeta[property] {
                if property == "og:image" {
                    if checkOgImageAspectRatio(n) {
                        // If the aspect ratio is 1:1, add the og:image to favicons
                        if content, found := attrMap["content"]; found {
                            if !contains(favicons, content) {
                                favicons = append(favicons, content)
                            }
                        }
                    }
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
            // If the favicon URL is a relative path, we should prepend the scheme and host of the URL
            for i, faviconURL := range favicons {
                favicons[i] = ensureAbsoluteURL(faviconURL, url)

        }
        return favicons, true
    }
    return nil, false
}

// checkOgImageAspectRatio checks the next 2 or 3 sibling nodes for og:image:width and og:image:height
// and verifies if the aspect ratio is 1:1 or if width and height are not specified.
// Returns true if the og:image should be added to favicons.
func checkOgImageAspectRatio(n *html.Node) bool {
    width, height := "", ""
    widthFound, heightFound := false, false
    for i, sibling := 0, n.NextSibling; sibling != nil && i < 3; sibling, i = sibling.NextSibling, i+1 {
        if sibling.Type == html.ElementNode && sibling.Data == "meta" {
            attrMap := extractAttributes(sibling.Attr)
            if property, found := attrMap["property"]; found {
                if property == "og:image:width" {
                    width = attrMap["content"]
                    widthFound = true
                } else if property == "og:image:height" {
                    height = attrMap["content"]
                    heightFound = true
                }
            }
        }
        if widthFound && heightFound {
            break
        }
    }
    // If width and height are found and equal, return true.
    return (width == height && width != "");
}