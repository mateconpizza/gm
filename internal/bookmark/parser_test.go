package bookmark

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testSingleBookmark() *Bookmark {
	return &Bookmark{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,tag1,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01T12:00:00Z",
		LastVisit: "2023-01-01T12:00:00Z",
		Favorite:  true,
	}
}

func TestExtractBlock(t *testing.T) {
	t.Parallel()
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
				"Failed for content: %v, startMarker: %s, endMarker: %s\nExpected: %q\nGot: %q\n",
				tt.content,
				tt.startMarker,
				tt.endMarker,
				tt.expected,
				result,
			)
		}
	}
}

func TestRecordIsValid(t *testing.T) {
	t.Parallel()
	validBookmark := testSingleBookmark()
	err := Validate(validBookmark)
	assert.NoError(t, err, "expected valid bookmark to be valid")
	// invalid bookmark
	invalidBookmark := testSingleBookmark()
	invalidBookmark.Title = ""
	invalidBookmark.URL = ""
	err = Validate(invalidBookmark)
	assert.Error(t, err, "expected invalid bookmark to be invalid")
}

//nolint:dupword //test
func TestParseTags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid tags",
			input:    "tag1, tag2, tag3 tag",
			expected: "tag,tag1,tag2,tag3,",
		},
		{
			name:     "duplicate tags",
			input:    "tag2, tag2 tag1, tag1, tag1",
			expected: "tag1,tag2,",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "notag",
		},
		{
			name:     "single tag",
			input:    "tag",
			expected: "tag,",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tt := test
			got := ParseTags(tt.input)
			assert.Equal(t, tt.expected, got, "expected %s, got %s", tt.expected, got)
		})
	}
}
