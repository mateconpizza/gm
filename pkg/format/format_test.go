package format

import (
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
