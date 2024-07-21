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

    // validate the URL
    isValid := validateURL(url)
    if !isValid {
        cacheErr := fmt.Errorf("retrieveHTML failed to validate URL")
        f.addToCache(url, nil, cacheErr)
        return nil, cacheErr
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

    doc, err := html.Parse(httpResp.Body)

    if err != nil {
        cacheErr := fmt.Errorf("retrieveHTML failed to parse HTML: %v", err)
        f.addToCache(url, nil, cacheErr)
        return nil, cacheErr
    }

    headNode := findHeadNode(doc)
    if headNode == nil {
        return nil, fmt.Errorf("no <head> element found")
    }

    f.addToCache(url, headNode, nil)
    return headNode, nil
}

func findHeadNode(n *html.Node) *html.Node {
    if n.Type == html.ElementNode && n.Data == "head" {
        return n
    }

    for c := n.FirstChild; c != nil; c = c.NextSibling {
        if headNode := findHeadNode(c); headNode != nil {
            return headNode
        }
    }

    return nil
}




func validateURL(url string) bool {
	_, err := Url.Parse(url)
	return err == nil
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

