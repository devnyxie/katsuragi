package katsuragi

import (
	"testing"
)

// invalid URL
func TestGetFavicons_InvalidURL(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()

	htmlTemplate := ``
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	_, err := f.GetFavicons("255.255.255.0")

	if err == nil {
		t.Fatalf("Expected an error, got none")
	}
}

// no favicon tags
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

// "icon" tag
func TestGetFavicons_IconTag(t *testing.T) {
    htmlContent := `<html><head><link rel="icon" href="/favicon.ico"></head><body></body></html>`
    mockServer := MockServer(t, htmlContent)
    defer mockServer.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    defer f.ClearCache()

    favicons, err := f.GetFavicons(mockServer.URL)
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
    if len(favicons) != 1 {
        t.Fatalf("Expected to find 1 favicon, found %d", len(favicons))
    }
}

// "apple-touch-icon" tag
func TestGetFavicons_AppleTouchIconTag(t *testing.T) {
    htmlContent := `<html><head><link rel="apple-touch-icon" href="/apple-touch-icon.png"></head><body></body></html>`
    mockServer := MockServer(t, htmlContent)
    defer mockServer.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    defer f.ClearCache()

    favicons, err := f.GetFavicons(mockServer.URL)
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
    if len(favicons) != 1 {
        t.Fatalf("Expected to find 1 favicon, found %d", len(favicons))
    }
}

// "og:image" tag
func TestGetFavicons_OgImageTag_NoSizeSpecified(t *testing.T) {
    htmlContent := `<html><head><meta property="og:image" content="og-image.png"></head><body></body></html>`
    mockServer := MockServer(t, htmlContent)
    defer mockServer.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    defer f.ClearCache()

    favicons, err := f.GetFavicons(mockServer.URL)

    if err == nil {
        t.Fatalf("Expected an error, got none")
    }

    if len(favicons) != 0 {
        t.Fatalf("Expected to find 0 favicons, found %d", len(favicons))
    }
}

// "og:image" tag with non-square aspect ratio specified
func TestGetFavicons_OgImageTag_NonSquare(t *testing.T) {
    htmlContent := `<html><head><meta property="og:image" content="og-image.png"><meta property="og:image:type" content="image/png"><meta property="og:image:width" content="1200"><meta property="og:image:height" content="630"></head><body></body></html>`
    mockServer := MockServer(t, htmlContent)
    defer mockServer.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    defer f.ClearCache()

    favicons, err := f.GetFavicons(mockServer.URL)
    if err == nil {
        t.Fatalf("Expected an error due to non-square aspect ratio, got none")
    }
    if len(favicons) != 0 {
        t.Fatalf("Expected to find 0 favicons, found %d", len(favicons))
    }
}

// "og:image" tag with square aspect ratio specified
func TestGetFavicons_OgImageTag_Square(t *testing.T) {
    htmlContent := `<html><head><meta property="og:image" content="og-image.png"><meta property="og:image:type" content="image/png"><meta property="og:image:width" content="1200"><meta property="og:image:height" content="1200"></head><body></body></html>`
    mockServer := MockServer(t, htmlContent)
    defer mockServer.Close()

    f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
    defer f.ClearCache()

    favicons, err := f.GetFavicons(mockServer.URL)
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
    if len(favicons) != 1 {
        t.Fatalf("Expected to find 1 favicon, found %d", len(favicons))
    }
}
