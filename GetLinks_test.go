package katsuragi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetLinks(t *testing.T) {
    tests := []struct {
        name              string
		category 		  string
		url 			  string
        responseBody      func(serverURL string) string // Function to generate response body dynamically
        expectedErr       string
        expectedLinks     []string
    }{
		{
			name: "all",
			category: "all",
			responseBody: func(serverURL string) string {
				return fmt.Sprintf(`<html><body>
					<a href="%s/internal1">Internal 1</a>
					<a href="%s/internal2">Internal 2</a>
					<a href="http://external.com">External</a>
					</body></html>`, serverURL, serverURL)
			},
			expectedErr:   "",
			expectedLinks: []string{"<serverURL>/internal1", "<serverURL>/internal2", "http://external.com"},
		},
        {
            name: "internal",
			category: "internal",
            responseBody: func(serverURL string) string {
                return fmt.Sprintf(`<html><body>
                    <a href="%s/internal1">Internal 1</a>
                    <a href="%s/internal2">Internal 2</a>
                    </body></html>`, serverURL, serverURL)
            },
            expectedErr:   "",
            expectedLinks: []string{"<serverURL>/internal1", "<serverURL>/internal2"},
        },
		{
			name: "external",
			category: "external",
			responseBody: func(serverURL string) string {
				return fmt.Sprintf(`<html><body>
					<a href="%s/internal1">Internal 1</a>
					<a href="http://external.com">External</a>
					</body></html>`, serverURL)
			},
			expectedErr:   "",
			expectedLinks: []string{"http://external.com"},
		},
		{
			name: "no category",
			category: "",
			responseBody: func(serverURL string) string {
				return fmt.Sprintf(`<html><body>
					<a href="%s/internal1">Internal 1</a>
					<a href="%s/internal2">Internal 2</a>
					<a href="http://external.com">External</a>
					</body></html>`, serverURL, serverURL)
			},
			expectedErr:   "",
			expectedLinks: []string{"<serverURL>/internal1", "<serverURL>/internal2", "http://external.com"},
		},
		// bad url
		{
			name: "bad url",
			category: "all",
			url: "http:/",
			responseBody: func(serverURL string) string {
				return ""
			},
			expectedErr: "Get \"http:/\": http: no Host in request URL",
			expectedLinks: []string{},
		},
		// broken link in html
		{
			name: "good and invalid links in html",
			category: "all",
			responseBody: func(serverURL string) string {
				return fmt.Sprintf(`<html><body>
					<a href=":/|%s/internal1">Internal 1</a>
					<a href=":htpexternal.com">External</a>
					<a href="http://external2.com">External</a>
					<a href="/test">External</a>
					</body></html>`, serverURL)
			},
			expectedErr:   "",
			expectedLinks: []string{"http://external2.com", "<serverURL>/test"},
		},
		// no links in html
		{
			name: "no links in html",
			category: "all",
			responseBody: func(serverURL string) string {
				return "<html><body></body></html>"
			},
			expectedErr: "GetLinks failed to find any links in HTML",
			expectedLinks: []string{},
		},	
	}

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
            server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                fmt.Fprint(w, tt.responseBody(server.URL))
            }))
            defer server.Close()

			// Replace "<serverURL>" in expectedLinks with the actual server.URL before assertions
			for i, link := range tt.expectedLinks {
                tt.expectedLinks[i] = strings.Replace(link, "<serverURL>", server.URL, -1)
            }

            fetcher := NewFetcher(&FetcherProps{Timeout: 3000, CacheCap: 10})

			var links []string
			var err error

			if tt.url != "" {
				links, err = fetcher.GetLinks(GetLinksProps{Url: tt.url, Category: tt.category})
			} else {
				links, err = fetcher.GetLinks(GetLinksProps{Url: server.URL, Category: tt.category})
			}

			fmt.Println("Server URL: ", server.URL)
       
            // Test assertions follow
			if err != nil && tt.expectedErr == "" {
				t.Errorf("Expected no error, got %v", err)
			}
			if err == nil && tt.expectedErr != "" {
				t.Errorf("Expected error %v, got none", tt.expectedErr)
			}
			// compare errors
			if err != nil && tt.expectedErr != "" {
				if err.Error() != tt.expectedErr {
					t.Errorf("Expected error %q, got %q", tt.expectedErr, err.Error())
				}
			}
			if len(links) != len(tt.expectedLinks) {
				t.Errorf("Expected %d links, got %d. Links: %s", len(tt.expectedLinks), len(links), links)
			}

			// compare expected links with actual links
			fmt.Println("Result: ", links)
			if len(links) > 0 {
				for i, link := range links {
					if link != tt.expectedLinks[i] {
						t.Errorf("Expected link %s, got %s", tt.expectedLinks[i], link)
					}
				}
			}

        })
    }
}

