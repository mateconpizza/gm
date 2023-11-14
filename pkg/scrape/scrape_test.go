package scrape

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
			expected: TitleDefault,
			wantErr:  false,
		},
		{
			name:     "HTTPError",
			url:      "http://invalid-url",
			server:   nil,
			expected: TitleDefault,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.server != nil {
				defer tt.server.Close()
				tt.url = tt.server.URL
			}

			got, err := GetTitle(tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetTitle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.expected {
				t.Errorf("GetTitle() = %v, want %v", got, tt.expected)
			}
		})
	}
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
			expected: DescDefault,
			wantErr:  false,
		},
		{
			name:     "HTTPError",
			url:      "http://invalid-url",
			server:   nil,
			expected: DescDefault,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.server != nil {
				defer tt.server.Close()
				tt.url = tt.server.URL
			}

			got, err := GetDescription(tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDescription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.expected {
				t.Errorf("GetDescription() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func createTestServer(responseBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, responseBody)
	}))
}
