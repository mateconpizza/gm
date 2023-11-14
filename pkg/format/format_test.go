package format

import (
	"slices"
	"testing"
)

func TestShortenString(t *testing.T) {
	input := "This is a long string"
	maxLength := 10
	expected := "This is..."
	result := ShortenString(input, maxLength)

	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFormatLine(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		v      string
		c      string
		want   string
	}{
		{
			name:   "NoColor",
			prefix: "Prefix",
			v:      "Value",
			c:      "",
			want:   "PrefixValue\n",
		},
		{
			name:   "WithColor",
			prefix: "Prefix",
			v:      "Value",
			c:      "\x1b[31m",
			want:   "\x1b[31mPrefixValue\x1b[0m\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLine(tt.prefix, tt.v, tt.c)
			if got != tt.want {
				t.Errorf("FormatLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseUniqueStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		sep      string
		expected []string
	}{
		{
			name:     "EmptyInput",
			input:    []string{},
			sep:      ",",
			expected: []string{},
		},
		{
			name:     "SingleTag",
			input:    []string{"tag1"},
			sep:      ",",
			expected: []string{"tag1"},
		},
		{
			name:     "MultipleTags",
			input:    []string{"tag1, tag2, tag3, tag3"},
			sep:      ",",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "RepeatedTags",
			input:    []string{"tag1, tag2, tag1, tag3, tag2"},
			sep:      ",",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "WhitespaceTags",
			input:    []string{"  tag1  ,  tag2  , tag3  "},
			sep:      ",",
			expected: []string{"tag1", "tag2", "tag3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseUniqueStrings(tt.input, tt.sep)
			if !slices.Equal(tt.expected, got) {
				t.Errorf("ParseUniqueStrings() = %v, want %v", got, tt.expected)
			}
		})
	}
}
