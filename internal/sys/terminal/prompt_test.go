package terminal

import (
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/stretchr/testify/assert"
)

func TestTermGetUserInput(t *testing.T) {
	t.Run("confirm", func(t *testing.T) {
		t.Parallel()

		opts := []string{"yes", "no"}
		input := "yes\n"
		mockInput := strings.NewReader(input)
		mockOutput := &strings.Builder{}
		result, err := getUserInputWithAttempts(mockInput, mockOutput, "Proceed?", opts, "no")
		assert.NoError(t, err)
		assert.Equal(t, "yes", result, "Expected 'yes' but got %s", result)
	})
	t.Run("default", func(t *testing.T) {
		t.Parallel()

		opts := []string{"yes", "no"}
		input := "\n"
		mockInput := strings.NewReader(input)
		mockOutput := &strings.Builder{}
		result, err := getUserInputWithAttempts(mockInput, mockOutput, "Proceed?", opts, "no")
		assert.NoError(t, err)
		assert.Equal(t, "no", result)
	})
	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		opts := []string{"yes", "no"}
		input := "invalid\n"
		mockInput := strings.NewReader(input)
		mockOutput := &strings.Builder{}
		result, err := getUserInputWithAttempts(mockInput, mockOutput, "Proceed?", opts, "no")
		assert.Error(t, err)
		assert.Empty(t, result)
	})
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

			result := fmtChoicesWithDefaultColor(tt.opts, tt.def)
			for i := range len(result) {
				result[i] = color.ANSICodeRemover(result[i])
			}

			assert.Equal(t, tt.want, result)
		})
	}
}
