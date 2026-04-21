package txt

import (
	"testing"
)

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
		{
			name: "preserves empty lines",
			input: `Line one
Line two

Line four`,
			strLen: 50,
			expected: []string{
				"Line one",
				"Line two",
				"",
				"Line four",
			},
		},
		{
			name: "preserves multiple empty lines",
			input: `First


Third`,
			strLen: 50,
			expected: []string{
				"First",
				"",
				"",
				"Third",
			},
		},
		{
			name: "wraps long paragraphs",
			input: `A utility library based on Go 1.18+ generics that makes it easier to work with slices.

See also samber/do`,
			strLen: 40,
			expected: []string{
				"A utility library based on Go 1.18+",
				"generics that makes it easier to work",
				"with slices.",
				"",
				"See also samber/do",
			},
		},
		{
			name:   "handles trailing newline",
			input:  "Line one\nLine two\n",
			strLen: 50,
			expected: []string{
				"Line one",
				"Line two",
				"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := SplitIntoChunks(tt.input, tt.strLen)
			if len(result) != len(tt.expected) {
				t.Errorf(
					"SplitIntoChunks(%q, %d) length = %d, expected %d",
					tt.input,
					tt.strLen,
					len(result),
					len(tt.expected),
				)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf(
						"SplitIntoChunks(%q, %d)[%d] = %q, expected %q",
						tt.input,
						tt.strLen,
						i,
						result[i],
						tt.expected[i],
					)
				}
			}
		})
	}
}
