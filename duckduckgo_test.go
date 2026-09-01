package katsuragi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDuckDuckGoSearchProvider_Search(t *testing.T) {
	tests := []struct {
		name        string
		responseBody string
		statusCode  int
		expectedErr string
		expected    []string
	}{
		{
			name: "single result",
			responseBody: `<html><body>
				<a class="result__a" href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fallegro.pl%2F&amp;rut=abc">Allegro</a>
			</body></html>`,
			statusCode: http.StatusOK,
			expected:   []string{"https://allegro.pl/"},
		},
		{
			name: "multiple results, deduplicated, capped at MaxResults",
			responseBody: `<html><body>
				<a class="result__a" href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fone.example%2F">One</a>
				<a class="result__a" href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Ftwo.example%2F">Two</a>
				<a class="result__a" href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fone.example%2F">One again</a>
			</body></html>`,
			statusCode: http.StatusOK,
			expected:   []string{"https://one.example/", "https://two.example/"},
		},
		{
			name:        "no results",
			responseBody: `<html><body>No results.</body></html>`,
			statusCode:  http.StatusOK,
			expectedErr: `duckduckgo search: no results for "brand"`,
		},
		{
			name:        "non-200 status",
			responseBody: "",
			statusCode:  http.StatusServiceUnavailable,
			expectedErr: "duckduckgo search: unexpected status 503 Service Unavailable",
		},
		{
			name: "non-duckduckgo href passed through unchanged",
			responseBody: `<html><body>
				<a class="result__a" href="https://direct.example/page">Direct</a>
			</body></html>`,
			statusCode: http.StatusOK,
			expected:   []string{"https://direct.example/page"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.statusCode)
				_, _ = w.Write([]byte(test.responseBody))
			}))
			defer server.Close()

			provider := &DuckDuckGoSearchProvider{BaseURL: server.URL}
			results, err := provider.Search(context.Background(), "brand")

			if err != nil {
				if test.expectedErr == "" {
					t.Fatalf("expected no error, got: %v", err)
				}
				if err.Error() != test.expectedErr {
					t.Fatalf("expected error %q, got: %v", test.expectedErr, err)
				}
				return
			}
			if test.expectedErr != "" {
				t.Fatalf("expected error %q, got none", test.expectedErr)
			}

			if len(results) != len(test.expected) {
				t.Fatalf("expected %d results, got %d: %v", len(test.expected), len(results), results)
			}
			for i, want := range test.expected {
				if results[i] != want {
					t.Fatalf("result[%d]: expected %q, got %q", i, want, results[i])
				}
			}
		})
	}
}

func TestResolveDuckDuckGoHref(t *testing.T) {
	tests := []struct {
		name     string
		href     string
		expected string
	}{
		{
			name:     "wrapped redirect",
			href:     "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath&rut=x",
			expected: "https://example.com/path",
		},
		{
			name:     "absolute non-duckduckgo href",
			href:     "https://example.com/page",
			expected: "https://example.com/page",
		},
		{
			name:     "empty href",
			href:     "",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolveDuckDuckGoHref(test.href)
			if got != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, got)
			}
		})
	}
}
