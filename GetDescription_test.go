package katsuragi

import (
	"fmt"
	"testing"
)

// invalid URL
func TestGetDescription_InvalidURL(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()

	htmlTemplate := ``
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	_, err := f.GetDescription("255.255.255.0")

	if err == nil {
		t.Fatalf("Expected an error, got none")
	}
}

// empty HTML template
func TestGetDescription_EmptyHTML(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()

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

	_, err := f.GetDescription(mockServer.URL)

	if err != nil {
		expectedErrorMessage := "GetDescription failed to find description in HTML"
		if err.Error() != expectedErrorMessage {
			t.Fatalf("Expected error message '%s', got '%s'", expectedErrorMessage, err.Error())
		}
	} else {
		t.Fatalf("No error was returned")
	}
}

// empty description
func TestGetDescription_EmptyDescriptions(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()

	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
		    <description></description>
			<meta name="description" content="">
			<meta property="og:description" content="">
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	_, err := f.GetDescription(mockServer.URL)

	if err != nil {
		expectedErrorMessage := "GetDescription failed to find description in HTML"
		if err.Error() != expectedErrorMessage {
			t.Fatalf("Expected error message '%s', got '%s'", expectedErrorMessage, err.Error())
		}
	} else {
		t.Fatalf("No error was returned")
	}
}

//<meta name="description" content="Example Description">
func TestGetDescription_MetaNameDescription(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<meta name="description" content="Example Description">
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	result, err := f.GetDescription(mockServer.URL)

	fmt.Println(result)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "Example Description" {
		t.Fatalf("Expected description 'Example Description', got '%s'", result)
	}
}

//<meta property="og:description" content="Example Description">
func TestGetDescription_MetaPropertyOgDescription(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<meta property="og:description" content="Example Description">
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	result, err := f.GetDescription(mockServer.URL)

	fmt.Println(result)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "Example Description" {
		t.Fatalf("Expected description 'Example Description', got '%s'", result)
	}
}

//<meta name="twitter:description" content="Example Description">
func TestGetDescription_MetaNameTwitterDescription(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
	defer f.ClearCache()
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<meta name="twitter:description" content="Example Description">
		</head>
		<body>
		</body>
	</html>
	`
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	result, err := f.GetDescription(mockServer.URL)

	fmt.Println(result)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "Example Description" {
		t.Fatalf("Expected description 'Example Description', got '%s'", result)
	}
}