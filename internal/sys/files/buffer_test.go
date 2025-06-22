package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	te := &TextEditor{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, te.Diff(tt.inputA, tt.inputB))
		})
	}
}
