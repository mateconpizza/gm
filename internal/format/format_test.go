package format

import (
	"reflect"
	"testing"
)

func TestShortenString(t *testing.T) {
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
		n := len(r)
		if n != tt.length && r != tt.expected {
			t.Errorf("Expected %s, but got %s", tt.expected, r)
		}
	}
}

func TestUnique(t *testing.T) {
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

		if !reflect.DeepEqual(items, tt.expected) {
			t.Errorf("expected %v, got %v", tt.expected, items)
		}
	}
}
