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
			got := Line(tt.prefix, tt.v, tt.c)
			if got != tt.want {
				t.Errorf("FormatLine() = %v, want %v", got, tt.want)
			}
		})
	}
}
