package parser

import (
	"slices"
	"testing"
)

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
			got := Tags(tt.input)
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
		got := uniqueTags(tt.input)
		if !slices.Equal(tt.expected, got) {
			t.Errorf("uniqueTags(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
