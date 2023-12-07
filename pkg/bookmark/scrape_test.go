package bookmark

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testScrapeFunction(t *testing.T, getTitleOrDesc func(string) (string, error), tests []struct {
	name     string
	url      string
	server   *httptest.Server
	expected string
	wantErr  bool
},
) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.server != nil {
				defer tt.server.Close()
				tt.url = tt.server.URL
			}

			got, err := getTitleOrDesc(tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("%s() error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}

			if got != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestGetTitle(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		server   *httptest.Server
		expected string
		wantErr  bool
	}{
		{
			name:     "ValidTitle",
			url:      "http://example.com",
			server:   createTestServer(`<title>Test Title</title>`),
			expected: "Test Title",
			wantErr:  false,
		},
		{
			name:     "NoTitleTag",
			url:      "http://example.com",
			server:   createTestServer(`<h1>Test Heading</h1>`),
			expected: DefaultTitle,
			wantErr:  false,
		},
		{
			name:     "HTTPError",
			url:      "http://invalid-url",
			server:   nil,
			expected: DefaultTitle,
			wantErr:  true,
		},
	}

	testScrapeFunction(t, FetchTitle, tests)
}

func TestGetDescription(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		server   *httptest.Server
		expected string
		wantErr  bool
	}{
		{
			name:     "ValidDescription",
			url:      "http://example.com",
			server:   createTestServer("<html><head><meta name=\"description\" content=\"Test Description\"></head></html>"),
			expected: "Test Description",
			wantErr:  false,
		},
		{
			name:     "NoDescriptionMetaTag",
			url:      "http://example.com",
			server:   createTestServer(`<h1>Test Heading</h1>`),
			expected: DefaultDesc,
			wantErr:  false,
		},
		{
			name:     "HTTPError",
			url:      "http://invalid-url",
			server:   nil,
			expected: DefaultDesc,
			wantErr:  true,
		},
	}

	testScrapeFunction(t, FetchDescription, tests)
}

func createTestServer(responseBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, responseBody)
	}))
}
