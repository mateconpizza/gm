package terminal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTermGetUserInput(t *testing.T) {
	t.Parallel()
	opts := []string{"yes", "no"}
	// test confirm
	input := "yes\n"
	mockInput := strings.NewReader(input)
	result := getUserInput(mockInput, "Proceed?", opts, "no")
	assert.Equal(t, "yes", result, "Expected 'yes' but got %s", result)
	// test default
	input = "\n"
	mockInput = strings.NewReader(input)
	result = getUserInput(mockInput, "Proceed?", opts, "no")
	assert.Equal(t, "no", result)
	// test invalid
	input = "invalid\n"
	mockInput = strings.NewReader(input)
	result = getUserInput(mockInput, "Proceed?", opts, "no")
	assert.Empty(t, result)
}

func TestTermGetQueryFromPipe(t *testing.T) {
	t.Parallel()
	input := "hello\n"
	mockInput := strings.NewReader(input)
	result := getQueryFromPipe(mockInput)
	assert.Equal(t, input, result)
}

func TestTermFmtChoicesWithDefault(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		opts []string
		def  string
		want []string
	}{
		{
			name: "with default 'no'",
			opts: []string{"yes", "no"},
			def:  "n",
			want: []string{"yes", "No"},
		},
		{
			name: "with default 'yes'",
			opts: []string{"yes", "no"},
			def:  "y",
			want: []string{"no", "Yes"},
		},
		{
			name: "no default",
			opts: []string{"yes", "no"},
			def:  "",
			want: []string{"yes", "no"},
		},
		{
			name: "empty options",
			opts: []string{},
			def:  "",
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := fmtChoicesWithDefault(tt.opts, tt.def)
			assert.Equal(t, tt.want, result)
		})
	}
}
