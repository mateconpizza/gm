package format

import (
	"reflect"
	"testing"
)

func TestShortenString(t *testing.T) {
	input := "This is a long string"
	maxLength := 10
	expected := "This is..."
	result := Shorten(input, maxLength)

	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
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
