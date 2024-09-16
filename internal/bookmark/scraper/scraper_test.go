package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createTestServer(responseBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintln(w, responseBody)
		if err != nil {
			panic(err)
		}
	}))
}

func testScrapeFn(t *testing.T, getTitleOrDesc func(string) (string, error), tests []struct {
	name     string
	url      string
	server   *httptest.Server
	expected string
},
) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.server != nil {
				defer tt.server.Close()
				tt.url = tt.server.URL
			}

			got, err := getTitleOrDesc(tt.url)
			if err != nil {
				t.Errorf("%s() error = %v", tt.name, err)

				return
			}

			if got != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestTitle(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		server   *httptest.Server
		expected string
	}{
		{
			name:     "ValidTitle",
			url:      "http://example.com",
			server:   createTestServer(`<title>Test Title</title>`),
			expected: "Test Title",
		},
		{
			name:     "NoTitleTag",
			url:      "http://example.com",
			server:   createTestServer(`<h1>Test Heading</h1>`),
			expected: defaultTitle,
		},
		{
			name:     "NoValidURL",
			url:      "http://invalid-url",
			server:   createTestServer(``),
			expected: defaultTitle,
		},
	}

	testScrapeFn(t, func(url string) (string, error) {
		sc := New(url)
		_ = sc.Scrape()

		return sc.Title(), nil
	}, tests)
}

//nolint:funlen //test
func TestDesc(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		server   *httptest.Server
		expected string
	}{
		{
			name: "ValidDescription",
			url:  "http://example.com",
			server: createTestServer(
				"<html><head><meta name=\"description\" content=\"Test Description\"></head></html>",
			),
			expected: "Test Description",
		},
		{
			name: "NoDescriptionMetaTag",
			url:  "http://example.com",
			server: createTestServer(
				"<html><head><title>Test Title</title></head></html>",
			),
			expected: defaultDesc,
		},
		{
			name: "ValidMetaDescription",
			url:  "http://example.com",
			server: createTestServer(
				"<html><head><meta name=\"description\" content=\"Another Test Description\"></head></html>",
			),
			expected: "Another Test Description",
		},
		{
			name: "MultipleDescriptionMetaTags",
			url:  "http://example.com",
			server: createTestServer(
				//nolint:lll //test
				`<html><head><meta name="description" content="First Description"><meta property="description" content="Second Description"></head></html>`,
			),
			expected: "First Description",
		},
		{
			name: "EmptyDescriptionContent",
			url:  "http://example.com",
			server: createTestServer(
				"<html><head><meta name=\"description\" content=\"\"></head></html>",
			),
			expected: defaultDesc,
		},
		{
			name: "DescriptionWithWhitespace",
			url:  "http://example.com",
			server: createTestServer(
				"<html><head><meta name=\"description\" content=\"  Description with spaces  \"></head></html>",
			),
			expected: "Description with spaces",
		},
		{
			name:     "InvalidDescription",
			url:      "http://example.com",
			server:   createTestServer(``),
			expected: defaultDesc,
		},
	}

	testScrapeFn(t, func(url string) (string, error) {
		sc := New(url)
		_ = sc.Scrape()

		return sc.Desc(), nil
	}, tests)
}
