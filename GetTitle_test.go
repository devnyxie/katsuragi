package katsuragi

import (
	"testing"
)

// invalid URL
func TestGetTitle_InvalidURL(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()

	htmlTemplate := ``
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	_, err := f.GetTitle("255.255.255.0")

	if err == nil {
		t.Fatalf("Expected an error, got none")
	}
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
	defer f.ClearCache()
	
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
	defer f.ClearCache()
	
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
	defer f.ClearCache()
	
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
	defer f.ClearCache()
	
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

// empty titles
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
	defer f.ClearCache()
	
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

// all titles exist
func TestGetTitle_AllTitlesExist(t *testing.T) {
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<title>Example Title</title>
			<meta name="title" content="Example Title">
			<meta property="og:title" content="Example Title">
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()
	
	title, err := f.GetTitle(mockServer.URL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if title != "Example Title" {
		t.Fatalf("Expected title 'Example Title', got '%s'", title)
	}
}