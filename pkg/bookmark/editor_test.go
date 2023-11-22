package bookmark

import (
	"testing"
)

func TestExtractBlock(t *testing.T) {
	tests := []struct {
		startMarker string
		endMarker   string
		expected    string
		content     []string
	}{
		{
			content: []string{
				"Line 1",
				"## start marker",
				"Content to extract",
				"More content",
				"## end marker",
				"Line after end marker",
			},
			startMarker: "## start marker",
			endMarker:   "## end marker",
			expected:    "Content to extract\nMore content",
		},
		{
			content: []string{
				"Line 1",
				"## start marker",
				"Content to extract",
				"Only start marker, no end marker",
			},
			startMarker: "## start marker",
			endMarker:   "## end marker",
			expected:    "",
		},
	}

	for _, test := range tests {
		result := extractBlock(test.content, test.startMarker, test.endMarker)
		if result != test.expected {
			t.Errorf(
				"Failed for content: %v, startMarker: %s, endMarker: %s\nExpected: %s\nGot: %s\n",
				test.content,
				test.startMarker,
				test.endMarker,
				test.expected,
				result,
			)
		}
	}
}
