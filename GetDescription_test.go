package katsuragi

import (
	"testing"
)

func TestGetDescription(t *testing.T) {
	tests := []struct {
		name string
		url string
		mockupServerNeed bool
		responseBody string
		expectedErr string
		expectedRes string
		}{
		{
			name: "Invalid URL: No scheme",
			url: "255.255.255.0",
			mockupServerNeed: false,
			responseBody: "",
			expectedErr: "Get \"255.255.255.0\": unsupported protocol scheme \"\"",
			expectedRes: "",
		},
		{
			name: "Invalid URL: Empty",
			url: "",
			mockupServerNeed: false,
			responseBody: "",
			expectedErr: "Get \"\": unsupported protocol scheme \"\"",
			expectedRes: "",
		},
		{
			name: "No description tags",
			url: "",
			mockupServerNeed: true,
			responseBody: `
			<!DOCTYPE html>
			<html>
				<head>
				</head>
				<body>
				</body>
			</html>
			`,
			expectedErr: "GetDescription failed to find description in HTML",
			expectedRes: "",
		},
		{
			name: "Meta [name=description] tag",
			url: "",
			mockupServerNeed: true,
			responseBody: `
			<!DOCTYPE html>
			<html>
				<head>
					<meta name="description" content="Example Description">
				</head>
				<body>
				</body>
			</html>
			`,
			expectedErr: "",
			expectedRes: "Example Description",
		},
		{
			name: "Meta [property=og:description] tag",
			url: "",
			mockupServerNeed: true,
			responseBody: `
			<!DOCTYPE html>
			<html>
				<head>
					<meta property="og:description" content="Example Description">
				</head>
				<body>
				</body>
			</html>
			`,
			expectedErr: "",
			expectedRes: "Example Description",
		},
		{
			name: "Meta [name=twitter:description] tag",
			url: "",
			mockupServerNeed: true,
			responseBody: `
			<!DOCTYPE html>
			<html>
				<head>
					<meta name="twitter:description" content="Example Description">
				</head>
				<body>
				</body>
			</html>
			`,
			expectedErr: "",
			expectedRes: "Example Description",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			var err error

			f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
			defer f.ClearCache()
			mockServer := MockServer(t, tt.responseBody)
			defer mockServer.Close()

			if tt.mockupServerNeed {
				result, err = f.GetDescription(mockServer.URL)
			} else {
				result, err = f.GetDescription(tt.url)
			}
			
			// error validation
			if tt.expectedErr == "" && err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if tt.expectedErr != "" && err == nil {
				t.Fatalf("Expected error, got none")
			}
			if tt.expectedErr != "" && err.Error() != tt.expectedErr {
				t.Fatalf("Expected error %q, got %q", tt.expectedErr, err.Error())
			}

			// result validation
			if result != tt.expectedRes {
				t.Fatalf("Expected result `%s`, got %s", tt.expectedRes, result)
			}
		})
	}
}
	