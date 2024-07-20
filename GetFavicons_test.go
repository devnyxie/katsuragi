package katsuragi

import "testing"

func TestGetFavicons_NoFavicons(t *testing.T) {
    htmlContent := `<html><head><title>No Favicons Here</title></head><body></body></html>`
    mockServer := MockServer(t, htmlContent)
    defer mockServer.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    defer f.ClearCache()

    _, err := f.GetFavicons(mockServer.URL)
	//expected error 
	if err == nil {
		t.Fatalf("Expected an error, got none")
	}
}

// TestGetFavicons_MultipleFavicons tests fetching from a URL with multiple favicon links
func TestGetFavicons_MultipleFavicons(t *testing.T) {
    htmlContent := `<html><head>
        <link rel="icon" href="favicon.ico" sizes="16x16">
        <link rel="icon" href="favicon-32.png" sizes="32x32">
        <link rel="apple-touch-icon" href="apple-touch-icon.png" sizes="180x180">
        </head><body></body></html>`
    mockServer := MockServer(t, htmlContent)
    defer mockServer.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    defer f.ClearCache()

    favicons, err := f.GetFavicons(mockServer.URL)
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
    if len(favicons) != 3 {
        t.Fatalf("Expected to find 3 favicons, found %d", len(favicons))
    }
}

// TestGetFavicons_InvalidHTML tests fetching from a URL with invalid HTML content
func TestGetFavicons_InvalidHTML(t *testing.T) {
    htmlContent := `<html><head><title>Broken HTML`
    mockServer := MockServer(t, htmlContent)
    defer mockServer.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    defer f.ClearCache()

    _, err := f.GetFavicons(mockServer.URL)
    if err == nil {
        t.Fatalf("Expected an error due to invalid HTML, got none")
    }
}