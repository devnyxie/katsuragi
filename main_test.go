package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchFavicons(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprintln(w, `
                <head>
                    <link rel="apple-touch-icon" sizes="60x60" href="/assets/favicons/apple-touch-icon.png">
                    <link rel="icon" type="image/png" sizes="32x32" href="/assets/favicons/favicon-32x32.png">
                    <link rel="icon" type="image/png" sizes="16x16" href="/assets/favicons/favicon-16x16.png">
                    <link rel="manifest" href="/assets/favicons/site.webmanifest">
                    <link rel="mask-icon" href="/assets/favicons/safari-pinned-tab.svg" color="#5bbad5">
                </head>
            `)
			htmlData, err := os.ReadFile("assets/template.html")
			if err != nil {
				t.Error()
			}
			w.Write(htmlData)
		case "/favicon.svg":
			serveImage(w, "favicon.svg", t)
		case "/favicon.ico":
			serveImage(w, "favicon.ico", t)
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))

	defer mockServer.Close()

	options := Options{
		Concurrency: false,
		Validate:    true,
		MaxDepth:    2,
		ReturnType:  "first",
	}

	favicons, err := FetchFavicons(mockServer.URL, options)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(favicons) == 0 {
		t.Fatalf("Expected to find favicons, got none")
	}
}

// serveImage simulates serving an image file from the server
func serveImage(w http.ResponseWriter, filename string, t *testing.T) {
	faviconsDirPath := "assets/favicons/"
	faviconPath := faviconsDirPath + filename
	imageData, err := os.ReadFile(faviconPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Write(imageData)
}
