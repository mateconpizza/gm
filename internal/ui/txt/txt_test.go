package txt

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

func TestExtractBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		startMarker string
		endMarker   string
		expected    string
		content     []string
	}{
		{
			content: []string{
				"Line 1",
				"## start marker",
				"Content to extract",
				"More content",
				"## end marker",
				"Line after end marker",
			},
			startMarker: "## start marker",
			endMarker:   "## end marker",
			expected:    "Content to extract\nMore content",
		},
		{
			content: []string{
				"Line 1",
				"## start marker",
				"Content to extract",
				"Only start marker, no end marker",
			},
			startMarker: "## start marker",
			endMarker:   "## end marker",
			expected:    "",
		},
	}

	for _, tt := range tests {
		result := ExtractBlock(tt.content, tt.startMarker, tt.endMarker)
		if result != tt.expected {
			t.Errorf(
				"Failed for content: %v, startMarker: %s, endMarker: %s\nExpected: %q\nGot: %q\n",
				tt.content,
				tt.startMarker,
				tt.endMarker,
				tt.expected,
				result,
			)
		}
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inputA   []byte
		inputB   []byte
		expected string
	}{
		{
			name:     "No Changes",
			inputA:   []byte("line1\nline2\nline3"),
			inputB:   []byte("line1\nline2\nline3"),
			expected: "line1\nline2\nline3",
		},
		{
			name:     "Line Added",
			inputA:   []byte("line1\nline2"),
			inputB:   []byte("line1\nline2\nline3"),
			expected: "line1\nline2\n+line3",
		},
		{
			name:     "Line Removed",
			inputA:   []byte("line1\nline2\nline3"),
			inputB:   []byte("line1\nline3"),
			expected: "line1\n-line2\nline3",
		},
		{
			name:     "Line Modified",
			inputA:   []byte("line1\nline2\nline3"),
			inputB:   []byte("line1\nlineX\nline3"),
			expected: "line1\n-line2\n+lineX\nline3",
		},
		{
			name:     "Multiple Changes",
			inputA:   []byte("lineA\nlineB\nlineC\nlineD"),
			inputB:   []byte("lineA\nlineX\nlineC\nlineY"),
			expected: "lineA\n-lineB\n+lineX\nlineC\n-lineD\n+lineY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, Diff(tt.inputA, tt.inputB))
		})
	}
}
