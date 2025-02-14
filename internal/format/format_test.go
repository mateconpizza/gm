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
