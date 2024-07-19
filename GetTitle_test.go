package katsuragi

import (
	"os"
	"testing"
)

// valid URL
func TestGetTitle_ValidURL(t *testing.T) {
	htmlTemplate, err := os.ReadFile("testdata/template.html")
	if err != nil {
		t.Fatalf("Failed to read template.html: %v", err)
	}
	mockServer := MockServer(t, string(htmlTemplate))
	defer mockServer.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache() // empty the cache
	
	_, err = f.GetTitle(mockServer.URL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// invalid URL
func TestGetTitle_InvalidURL(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache() // empty the cache
	_, err := f.GetTitle("http://localhost:1234")
	if err != nil {
        // expected error
		expectedErrorMessage := "retrieveHTML could not reach the URL: Get \"http://localhost:1234\": dial tcp 127.0.0.1:1234: connect: connection refused"
        if err.Error() != expectedErrorMessage {
            t.Fatalf("Expected error message '%s', got '%s'", expectedErrorMessage, err.Error())
        } else {
			return
		}
	}
	t.Fatalf("No error was returned")
}

// <title>
func TestGetTitle_TitleTag(t *testing.T) {
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<title>Example Title</title>
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache() // empty the cache
	
	title, err := f.GetTitle(mockServer.URL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if title != "Example Title" {
		t.Fatalf("Expected title 'Example Title', got '%s'", title)
	}
}

// <meta name="title" content="">
func TestGetTitle_MetaNameTitle(t *testing.T) {
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<meta name="title" content="Example Title">
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache() // empty the cache
	
	title, err := f.GetTitle(mockServer.URL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if title != "Example Title" {
		t.Fatalf("Expected title 'Example Title', got '%s'", title)
	}
}

// <meta property="og:title" content="">
func TestGetTitle_MetaPropertyTitle(t *testing.T) {
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<meta property="og:title" content="Example Title">
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache() // empty the cache
	
	title, err := f.GetTitle(mockServer.URL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if title != "Example Title" {
		t.Fatalf("Expected title 'Example Title', got '%s'", title)
	}
}


// empty HTML template
func TestGetTitle_BadHTML(t *testing.T) {
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache() // empty the cache
	
	_, err := f.GetTitle(mockServer.URL)

	if err != nil {
		expectedErrorMessage := "GetTitle failed to find title in HTML"
		if err.Error() != expectedErrorMessage {
			t.Fatalf("Expected error message '%s', got '%s'", expectedErrorMessage, err.Error())
		}
	} else {
		t.Fatalf("No error was returned")
	}
}

// empty title
func TestGetTitle_EmptyTitle(t *testing.T) {
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<title></title>
			<meta name="title" content="">
			<meta property="og:title" content="">
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache() // empty the cache
	
	_, err := f.GetTitle(mockServer.URL)

	if err != nil {
		expectedErrorMessage := "GetTitle failed to find title in HTML"
		if err.Error() != expectedErrorMessage {
			t.Fatalf("Expected error message '%s', got '%s'", expectedErrorMessage, err.Error())
		}
	} else {
		t.Fatalf("No error was returned")
	}
}