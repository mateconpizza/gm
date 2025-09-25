package bookmark

import (
	"errors"
	"slices"
	"testing"
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

func TestRecordIsValid(t *testing.T) {
	t.Parallel()

	validBookmark := testSingleBookmark()
	err := Validate(validBookmark)
	if err != nil {
		t.Fatalf("unexpected error validating valid bookmark: %v", err)
	}

	// URL err
	bookmarkWithoutURL := testSingleBookmark()
	bookmarkWithoutURL.URL = ""
	err = Validate(bookmarkWithoutURL)
	if err == nil {
		t.Fatalf("unexpected error validating valid bookmark: %v", err)
	}
	if !errors.Is(err, ErrBookmarkURLEmpty) {
		t.Errorf("expected error to be %q, got %q", ErrBookmarkURLEmpty.Error(), err.Error())
	}

	// Tags err
	bookmarkWithoutTags := testSingleBookmark()
	bookmarkWithoutTags.Tags = ""
	err = Validate(bookmarkWithoutTags)
	if err == nil {
		t.Fatalf("unexpected error validating valid bookmark: %v", err)
	}
	if !errors.Is(err, ErrBookmarkTagsEmpty) {
		t.Errorf("expected error to be %q, got %q", ErrBookmarkTagsEmpty.Error(), err.Error())
	}
}

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
			input:    "tag2, tag2 tag1, tag1",
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
			if got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestUniqueItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    []string
		expected []string
	}{
		{
			input:    []string{"a", "b", "b", "B", "c"},
			expected: []string{"a", "b", "B", "c"},
		},
		{
			input:    []string{"a", "a", "a", "a", "a"},
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		got := UniqueTags(tt.input)
		if !slices.Equal(tt.expected, got) {
			t.Errorf("uniqueTags(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
