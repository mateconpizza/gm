package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected string
	}{
		{
			name:     "No Changes",
			a:        []byte("line1\nline2\nline3"),
			b:        []byte("line1\nline2\nline3"),
			expected: "line1\nline2\nline3",
		},
		{
			name:     "Line Added",
			a:        []byte("line1\nline2"),
			b:        []byte("line1\nline2\nline3"),
			expected: "line1\nline2\n+line3",
		},
		{
			name:     "Line Removed",
			a:        []byte("line1\nline2\nline3"),
			b:        []byte("line1\nline3"),
			expected: "line1\n-line2\nline3",
		},
		{
			name:     "Line Modified",
			a:        []byte("line1\nline2\nline3"),
			b:        []byte("line1\nlineX\nline3"),
			expected: "line1\n-line2\n+lineX\nline3",
		},
		{
			name:     "Multiple Changes",
			a:        []byte("lineA\nlineB\nlineC\nlineD"),
			b:        []byte("lineA\nlineX\nlineC\nlineY"),
			expected: "lineA\n-lineB\n+lineX\nlineC\n-lineD\n+lineY",
		},
	}

	te := &TextEditor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, te.Diff(tt.a, tt.b))
		})
	}
}
