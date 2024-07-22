package katsuragi

import (
	"fmt"
	"net/http"
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

    headNode, err := findHeadNode(doc)
    if err != nil {
        return nil, err
    }


    f.addToCache(url, headNode, nil)
    return headNode, nil
}

func findHeadNode(n *html.Node) (*html.Node, error) {
    // Check if the current node is the head node
    // 1. It should be an element node
    // 2. The tag name should be "head"
    // 3. It should have a parent node
    if n.Type == html.ElementNode && n.Data == "head" && n.Parent != nil && n.Parent.Data == "html" {
        if n.FirstChild != nil {
            return n, nil
        }
    }

    // Recursively search for the head node
    for c := n.FirstChild; c != nil; c = c.NextSibling {
        if headNode, err := findHeadNode(c); headNode != nil {
            return headNode, err
        }
    }

    return nil, fmt.Errorf("no <head> element found")
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

