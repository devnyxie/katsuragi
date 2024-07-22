package katsuragi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/html"
)

// test contains func
func TestContains(t *testing.T) {
    // does not contain
    if contains([]string{"a", "b", "c"}, "d") {
        t.Fatalf("Expected false, got true")
    }
    // contains
    if !contains([]string{"a", "b", "c"}, "b") {
        t.Fatalf("Expected true, got false")
    }
}

func TestRetrieveHTML(t *testing.T) {
    basicMockupResponse := "<html><head><title>Test</title></head><body><h1>test</h1></body></html>"
    tests := []struct {
        name    string
        url     string
        fetcherProps *FetcherProps
        mockupServerNeed bool
        mockupServerResStatusCode int
        mockupServerContentType string
        mockupServerResponseBody string
        expectedRes html.Node
        expectedErr string

    }{
        {
            name: "404 (Any bad status code)",
            mockupServerNeed: true,
            mockupServerResStatusCode: http.StatusNotFound,
            expectedErr: "retrieveHTML failed to fetch URL. HTTP Status: 404 Not Found",
        },
        {
            name: "Invalid URL Escape",
            url: "http://%gh&%$",
            mockupServerNeed: false,
            expectedErr: "parse \"http://%gh&%$\": invalid URL escape \"%gh\"",
        },
        {
            name: "Empty URL",
            url: "",
            mockupServerNeed: false,
            expectedErr: "Get \"\": unsupported protocol scheme \"\"",
        },
        {
            name: "Unreachable URL",
            url: "http://[::1]:9999",
            mockupServerNeed: false,
            expectedErr: "Get \"http://[::1]:9999\": dial tcp [::1]:9999: connect: connection refused",
        },
        {
            name: "User Agent",
            url: "",
            fetcherProps: &FetcherProps{Timeout: 3000, CacheCap: 10, UserAgent: "test-agent"},
            mockupServerNeed: true,
            mockupServerResStatusCode: http.StatusOK,
            mockupServerResponseBody: basicMockupResponse,
            mockupServerContentType: "text/html",
            expectedRes: html.Node{
                Type: html.ElementNode,
                Data: "head",
            },
        },
        {
            name: "JSON",
            url: "",
            fetcherProps: &FetcherProps{Timeout: 3000, CacheCap: 10},
            mockupServerNeed: true,
            mockupServerResStatusCode: http.StatusOK,
            mockupServerResponseBody: "{}",
            mockupServerContentType: "application/json",
            expectedErr: "retrieveHTML failed to fetch URL. Content-Type: application/json",
        },
        {
            name: "Valid URL",
            url: "https://www.google.com",
            fetcherProps: &FetcherProps{Timeout: 3000, CacheCap: 10},
            mockupServerNeed: false,
            expectedErr: "",
            expectedRes: html.Node{
                Type: html.ElementNode,
                Data: "head",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var result *html.Node
            var err error
            var server *httptest.Server

            f := NewFetcher(tt.fetcherProps)
            defer f.ClearCache()

            if tt.mockupServerNeed {
                // server with a status, content type, and body
                server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                    w.Header().Set("Content-Type", tt.mockupServerContentType)
                    w.WriteHeader(tt.mockupServerResStatusCode)
                    w.Write([]byte(tt.mockupServerResponseBody))
                }))
                defer server.Close()
                result, err = retrieveHTML(server.URL, f)
            } else {
                result, err = retrieveHTML(tt.url, f)
            }

            // wrapping in order to avoid nil point ref
            if err != nil {
                // wrong error
                errText := err.Error()
                if tt.expectedErr != errText {
                    t.Fatalf("Expected error %q, got %q", tt.expectedErr, errText)
                }
            }

            // no error
            if tt.expectedErr != "" && err == nil {
                t.Fatalf("Expected error, got none")
            }
            
            // result validation
            // if no result was expected, return
            if tt.expectedRes.FirstChild == nil {
                return
            }

            // if result was expected, but is nil
            if result == nil {
                t.Fatalf("Expected result, got nil")
            }

            // HTML Node Result validation
            // function returns and caches only the <head> node
            if tt.expectedRes.Data != result.Data {
                t.Fatalf("Expected result %q, got %q", tt.expectedRes.Data, result.Data)
            }
        })
    }
}