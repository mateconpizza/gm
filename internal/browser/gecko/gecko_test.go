package gecko

import "testing"

func TestIsNonGenericURL(t *testing.T) {
	testCases := []struct {
		url      string
		expected bool
	}{
		{"about:blank", true},
		{"apt:install", true},
		{"chrome://extensions", true},
		{"file:///path/to/file", true},
		{"place:bookmarks", true},
		{"vivaldi://settings", true},
		{"http://example.com", false},
		{"https://example.com", false},
		{"ftp://example.com", false},
		{"", false},
		{"chrome://", true},
		{"vivaldi://", true},
		{"chrome://settings/foo", true},
		{"vivaldi://extensions/bar", true},
	}

	for _, tc := range testCases {
		result := isNonGenericURL(tc.url)
		if result != tc.expected {
			println("Error for url:", tc.url)
			println("Expected:", tc.expected)
			println("Got:", result)
		}
	}
}
