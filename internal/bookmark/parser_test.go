package bookmark

import "testing"

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

	for _, tt := range tests {
		test := tt
		result := extractTextBlock(test.content, test.startMarker, test.endMarker)
		if result != tt.expected {
			t.Errorf(
				"Failed for content: %v, startMarker: %s, endMarker: %s\nExpected: %s\nGot: %s\n",
				tt.content,
				tt.startMarker,
				tt.endMarker,
				tt.expected,
				result,
			)
		}
	}
}

func TestExtractLineContent(t *testing.T) {
	tests := []struct {
		c        []string
		expected int
	}{
		{
			c: []string{
				"# Line 1",
				"Line 2",
				"               ",
				"# Line 3",
				"Line 4",
				"",
				"Line 5",
			},
			expected: 3,
		},
		{
			c: []string{
				"# Line 1",
				" Line 2",
				"# Line 3",
				"# Line 4",
				"# Line 5",
			},
			expected: 1,
		},
	}

	for _, test := range tests {
		temp := test
		got := ExtractContentLine(&temp.c)
		if len(got) != test.expected {
			t.Errorf("ExtractLineContent() = %v, want %v", got, test.expected)
		}
	}
}
