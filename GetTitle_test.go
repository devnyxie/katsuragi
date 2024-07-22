package katsuragi

import (
	"testing"
)

func TestGetTitle(t *testing.T) {
    tests := []struct {
        name            string
        url             string
        mockupServerNeed bool
        responseBody    string
        expectedErr     string
        expectedRes     string
    }{
        {
            name:            "Invalid URL: No scheme",
            url:             "255.255.255.0",
            mockupServerNeed: false,
            responseBody:    "",
            expectedErr:     "Get \"255.255.255.0\": unsupported protocol scheme \"\"",
            expectedRes:     "",
        },
        {
            name:            "Invalid URL: Empty",
            url:             "",
            mockupServerNeed: false,
            responseBody:    "",
            expectedErr:     "Get \"\": unsupported protocol scheme \"\"",
            expectedRes:     "",
        },
        {
            name:            "No title tags",
            url:             "",
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
            expectedErr: "GetTitle failed to find title in HTML",
            expectedRes: "",
        },
        {
            name:            "<title> tag",
            url:             "",
            mockupServerNeed: true,
            responseBody: `
            <!DOCTYPE html>
            <html>
                <head>
                    <title>Example Title</title>
                </head>
                <body>
                </body>
            </html>
            `,
            expectedErr: "",
            expectedRes: "Example Title",
        },
		{
			name:            "<meta name='title'> tag",
			url:             "",
			mockupServerNeed: true,
			responseBody: `
			<!DOCTYPE html>
			<html>
				<head>
					<meta name="title" content="Example Title">
				</head>
				<body>
				</body>
			</html>
			`,
			expectedErr: "",
			expectedRes: "Example Title",
		},
		{
			name:            "<meta property='og:title'> tag",
			url:             "",
			mockupServerNeed: true,
			responseBody: `
			<!DOCTYPE html>
			<html>
				<head>
					<meta property="og:title" content="Example Title">
				</head>
				<body>
				</body>
			</html>
			`,
			expectedErr: "",
			expectedRes: "Example Title",
		},
		{
			name:            "<meta name='twitter:title'> tag",
			url:             "",
			mockupServerNeed: true,
			responseBody: `
			<!DOCTYPE html>
			<html>
				<head>
					<meta name="twitter:title" content="Example Title">
				</head>
				<body>
				</body>
			</html>
			`,
			expectedErr: "",
			expectedRes: "Example Title",
		},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var result string
            var err error

            if tt.mockupServerNeed {
                f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
                defer f.ClearCache()
                mockServer := MockServer(t, tt.responseBody)
                defer mockServer.Close()
                result, err = f.GetTitle(mockServer.URL)
            } else {
                f := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})
                defer f.ClearCache()
                htmlTemplate := tt.responseBody
                mockServer := MockServer(t, htmlTemplate)
                defer mockServer.Close()
                result, err = f.GetTitle(tt.url)
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