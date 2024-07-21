package katsuragi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// RetrieveHTML tests
func TestRetrieveHTML_Success(t *testing.T) {
    html := "<html><head><title>Test</title></head><body></body></html>"
    server := MockServer(t, html)
    defer server.Close()
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    node, err := retrieveHTML(server.URL, f)
    if err != nil {
        t.Fatalf("retrieveHTML returned an error: %v", err)
    }
    if node == nil {
        t.Fatal("retrieveHTML returned nil node")
    }
}

func TestRetrieveHTML_NonHTMLContent(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte("{}"))
    }))
    defer server.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    _, err := retrieveHTML(server.URL, f)
    if err == nil {
        t.Fatal("Expected an error for non-HTML content, got none")
    }
}

func TestRetrieveHTML_404(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNotFound)
    }))
    defer server.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    _, err := retrieveHTML(server.URL, f)
    if err == nil {
        t.Fatal("Expected an error for non-200 status code, got none")
    }
}

// User Agent test
func TestRetrieveHTML_UserAgent(t *testing.T) {
    html := "<html><head><title>Test</title></head><body></body></html>"
    server := MockServer(t, html)

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10, UserAgent: "test-agent"})
    _, err := retrieveHTML(server.URL, f)

    if err != nil {
        t.Fatalf("retrieveHTML returned an error: %v", err)
    }
}

func TestRetrieveHTML_Requests(t *testing.T) {
    html := "<html><head><title>Test</title></head><body></body></html>"
    server := MockServer(t, html)
    defer server.Close()
    tests := []struct {
        name    string
        url     string
        wantErr bool
    }{
        {
            name:    "Valid URL",
            url:     server.URL,
            wantErr: false,
        },
        {
            name:    "Invalid URL",
            url:     "http://%gh&%$",
            wantErr: true,
        },
        {
            name:    "Empty URL",
            url:     "",
            wantErr: true,
        },
        {
            name:    "Unreachable URL",
            url:     "http://localhost:9999",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        // use NewFetcher and retrieveHTML
        t.Run(tt.name, func(t *testing.T) {
            f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
            _, err := retrieveHTML(tt.url, f)
            if tt.wantErr && err == nil {
                t.Fatalf("Expected error, got nil")
            }
            if !tt.wantErr && err != nil {
                t.Fatalf("Expected no error, got %v", err)
            }
        })
    
    }
}

//return json (not mockup server, custom)
func TestRetrieveHTML_JSON(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte("{}"))
    }))
    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    _, err := retrieveHTML(server.URL, f)
    if err == nil {
        t.Fatalf("Expected an error for non-HTML content, got none")
    }
}