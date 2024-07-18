package katsuragi

// Here are defined all required utils for the tests.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func MockServer(t *testing.T, htmlTemplate string) *httptest.Server {
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
			w.Header().Set("Content-Type", "text/html")
			html := []byte(htmlTemplate)
			w.Write(html)
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