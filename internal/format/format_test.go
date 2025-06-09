package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortenString(t *testing.T) {
	t.Parallel()
	test := []struct {
		input    string
		expected string
		length   int
	}{
		{
			input:    "This is a long string",
			length:   10,
			expected: "This is...",
		},
		{
			input:    "Neque porro quisquam est qui dolorem ipsum quia dolor sit amet, consectetur, adipisci velit...",
			length:   20,
			expected: "Neque porro quisq...",
		},
	}

	for _, tt := range test {
		r := Shorten(tt.input, tt.length)
		assert.Len(t, r, tt.length)
		assert.Equal(t, tt.expected, r)
	}
}

func TestUnique(t *testing.T) {
	t.Parallel()
	test := []struct {
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
	for _, tt := range test {
		items := Unique(tt.input)
		assert.Equal(t, tt.expected, items)
	}
}

//nolint:funlen //test
func TestSplitIntoChunks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		strLen   int
		expected []string
	}{
		{
			name:     "Single word shorter than strLen",
			input:    "hello",
			strLen:   10,
			expected: []string{"hello"},
		},
		{
			name:     "Multiple words fitting in one chunk",
			input:    "hello world",
			strLen:   11,
			expected: []string{"hello world"},
		},
		{
			name:     "Multiple words split into chunks",
			input:    "hello world this is a test",
			strLen:   10,
			expected: []string{"hello", "world this", "is a test"},
		},
		{
			name:     "Words split exactly at strLen",
			input:    "hello world",
			strLen:   5,
			expected: []string{"hello", "world"},
		},
		{
			name:     "Words split with spaces",
			input:    "hello world this is a test",
			strLen:   15,
			expected: []string{"hello world", "this is a test"},
		},
		{
			name:     "Multiple words with varying lengths",
			input:    "a bb ccc dddd eeeee",
			strLen:   10,
			expected: []string{"a bb ccc", "dddd eeeee"},
		},
		{
			name:     "Long sentence with multiple chunks",
			input:    "The quick brown fox jumps over the lazy dog",
			strLen:   10,
			expected: []string{"The quick", "brown fox", "jumps over", "the lazy", "dog"},
		},
		{
			name:   "Single character words",
			input:  "a b c d e f g h i j k l m n o p q r s t u v w x y z",
			strLen: 5,
			expected: []string{
				"a b c",
				"d e f",
				"g h i",
				"j k l",
				"m n o",
				"p q r",
				"s t u",
				"v w x",
				"y z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := SplitIntoChunks(tt.input, tt.strLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}
