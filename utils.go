package katsuragi

import (
	"fmt"
	"net/http"
	Url "net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// --- Generic utils ---
func retrieveHTML(url string, f *Fetcher) (*html.Node, error) {
    cachedValue, found, cachedErr := f.GetFromCache(url)
    if found {
        if cachedErr != nil {
            return nil, cachedErr
        }
        return cachedValue, nil
    }

    timeout := time.Duration(f.props.Timeout) * time.Millisecond

    var client *http.Client
    
    if f.props.UserAgent != "" {
        // Create a custom transport with the User-Agent
        transport := &http.Transport{
        }
        // Create a client with the custom transport and timeout
        client = &http.Client{
            Timeout: timeout,
            Transport: &UserAgentTransport{
                UserAgent: f.props.UserAgent,
                Transport: transport,
            },
        }
    } else {
        // Create a standard client with just the timeout if no User-Agent is specified
        client = &http.Client{
            Timeout: timeout,
        }
    }
    
    // Create a new request
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    // Make the request
    httpResp, err := client.Do(req)
    if err != nil {
        // Handle error
        return nil, err
    }
    defer httpResp.Body.Close()

    if httpResp.StatusCode != http.StatusOK {
        cacheErr := fmt.Errorf("retrieveHTML failed to fetch URL. HTTP Status: %v", httpResp.Status)
        f.addToCache(url, nil, cacheErr)
        return nil, cacheErr
    }

    // if the content type is not text/html, return an error
    contentType := httpResp.Header.Get("Content-Type")
    contentType = strings.ToLower(strings.Split(contentType, ";")[0])
    if contentType != "text/html" && contentType != "text/html; charset=utf-8" {
        cacheErr := fmt.Errorf("retrieveHTML failed to fetch URL. Content-Type: %v", contentType)
        f.addToCache(url, nil, cacheErr)
        return nil, cacheErr
    }

    doc, _ := html.Parse(httpResp.Body)
    // * Why we are not expecting an error here?
    // Before passing the body to the "html.Parse" function, we have already checked the HTTP status code and the content type of the response.
    // The "golang.org/x/net/html" package is very forgiving, and won't return any error even if we pass an empty string, so we can safely ignore the error here.
    // * Why we are not using the tokinezer instead in order to avoid the auto-correction of the parser that we do not need?
    // Tokenizing would increase the size of the code and the complexity of the implementation.

    // Remove script and style tags
    cleanHtml(doc)

    f.addToCache(url, doc, nil)
    return doc, nil
}

// cleanHtml removes script and style tags from the HTML
func cleanHtml(htmlres *html.Node) {
    var clean func(*html.Node)
    clean = func(n *html.Node) {
        var prev *html.Node
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "style") {
                if prev != nil {
                    prev.NextSibling = c.NextSibling
                } else {
                    n.FirstChild = c.NextSibling
                }
                if c.NextSibling != nil {
                    c.NextSibling.PrevSibling = prev
                }
            } else {
                prev = c
                clean(c)
            }
        }
    }
    clean(htmlres)
}

// extractAttributes returns a map of html attribute keys and values
func extractAttributes(attrs []html.Attribute) map[string]string {
    attrMap := make(map[string]string, len(attrs))
    for _, attr := range attrs {
        attrMap[attr.Key] = attr.Val
    }
    return attrMap
}

// contains checks if a string is in a slice
func contains(slice []string, value string) bool {
    for _, item := range slice {
        if item == value {
            return true
        }
    }
    return false
}



func ensureAbsoluteURL(href, base_url string) string {
    if !strings.HasPrefix(href, "http") {
        uri, _ := Url.Parse(base_url) // Note: Error handling is ignored here, consider handling it.
        if !strings.HasPrefix(href, "/") {
            href = "/" + href
        }
        return uri.Scheme + "://" + uri.Host + href
    }
    return href
}