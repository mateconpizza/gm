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
			t.Errorf("expected %s, but got %s", tt.expected, r)
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

func TestCounter(t *testing.T) {
	tests := []struct {
		expected map[string]int
		name     string
		input    []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: map[string]int{},
		},
		{
			name:     "single item",
			input:    []string{"apple"},
			expected: map[string]int{"apple": 1},
		},
		{
			name:     "multiple unique items",
			input:    []string{"apple", "banana", "orange"},
			expected: map[string]int{"apple": 1, "banana": 1, "orange": 1},
		},
		{
			name:     "duplicate items",
			input:    []string{"apple", "banana", "apple"},
			expected: map[string]int{"apple": 2, "banana": 1},
		},
		{
			name:     "mixed items",
			input:    []string{"apple", "banana", "orange", "banana", "apple"},
			expected: map[string]int{"apple": 2, "banana": 2, "orange": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Counter(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Counter(%v) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}
