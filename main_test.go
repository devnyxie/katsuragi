package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchFavicons(t *testing.T) {
	mockServer := MockServer(t)
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

	t.Logf("Found favicons: %d", len(favicons))

	if len(favicons) == 0 {
		t.Fatalf("Expected to find favicons, got none")
	}
}

func MockServer(t *testing.T) *httptest.Server {
	// get all filenames from testdata/favicons and store them in faviconsFilenames
	faviconsFilenames := []string{}
	faviconsDir, err := os.ReadDir("testdata/favicons")
	if err != nil {
		t.Error("Failed to read favicons directory:", err)
	}
	for _, entry := range faviconsDir {
		faviconsFilenames = append(faviconsFilenames, entry.Name())
	}
	// create a mock server that serves the HTML template with some rel tags and favicons
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		faviconIndex := -1
		for i, v := range faviconsFilenames {
			if "/"+v == r.URL.Path {
				faviconIndex = i
				break
			}
		}
		// create a route for each favicon if it exists in "testdata/favicons"
		if faviconIndex != -1 {
			filename := faviconsFilenames[faviconIndex]
			serveImage(w, filename, t)
		} else if r.URL.Path == "/" {
			htmlData, err := os.ReadFile("testdata/template.html")
			if err != nil {
				t.Error("Failed to read HTML template:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(htmlData)
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))

	return mockServer
}

// serveImage simulates serving an image file from the server
func serveImage(w http.ResponseWriter, filename string, t *testing.T) {
	faviconsDirPath := "testdata/favicons"
	faviconPath := faviconsDirPath + "/" + filename
	imageData, err := os.ReadFile(faviconPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Write(imageData)
}
