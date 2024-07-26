package katsuragi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func MockServer(t *testing.T, htmlTemplate string) *httptest.Server {
	// create a mock server that serves the HTML template with some rel tags and favicons
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
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
