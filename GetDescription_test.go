package katsuragi

import (
	"fmt"
	"testing"
)

// valid URL
func TestGetDescription_ValidURL(t *testing.T) {
	f := NewFetcher()
	defer f.Close() // empty the cache

	description := "Example Description"
	htmlTemplate := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
		<head>
			<meta name="description" content="%s">
		</head>
		<body>
		</body>
	</html>
	`, description)
	mockServer := MockServer(t, htmlTemplate)
	defer mockServer.Close()

	result, err := f.GetDescription(mockServer.URL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != description {
		t.Fatalf("Expected description '%s', got '%s'", description, result)
	}
}

// invalid URL
func TestGetDescription_InvalidURL(t *testing.T) {
	f := NewFetcher()
	defer f.Close() // empty the cache

	_, err := f.GetDescription("http://localhost:1234")
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

// empty HTML template
func TestGetDescription_BadHTML(t *testing.T) {
	f := NewFetcher()
	defer f.Close() // empty the cache

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
func TestGetDescription_EmptyDescription(t *testing.T) {
	f := NewFetcher()
	defer f.Close() // empty the cache

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

// <description> tag
func TestGetDescription_DescriptionTag(t *testing.T) {
	f := NewFetcher()
	defer f.Close() // empty the cache
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<description>Example Description</description>
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

// <meta name="description" content="">
func TestGetDescription_MetaNameDescription(t *testing.T) {
	f := NewFetcher()
	defer f.Close() // empty the cache
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

	description := "Example Description"
	result, err := f.GetDescription(mockServer.URL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != description {
		t.Fatalf("Expected description '%s', got '%s'", description, result)
	}
}

// <meta property="og:description" content="">
func TestGetDescription_MetaPropertyDescription(t *testing.T) {
	f := NewFetcher()
	defer f.Close() // empty the cache
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

	description := "Example Description"
	result, err := f.GetDescription(mockServer.URL)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != description {
		t.Fatalf("Expected description '%s', got '%s'", description, result)
	}
}