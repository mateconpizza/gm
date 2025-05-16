package terminal

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest //test
func TestHelper(t *testing.T) {
	t.Helper()
	NoColorEnv()
}

func TestTermPrompt(t *testing.T) {
	t.Parallel()
	question := "Enter your favorite language: "
	input := "golang\n"
	mockInput := strings.NewReader(input)
	term := New(WithReader(mockInput))
	result := term.Prompt(question)
	assert.Equal(t, "golang", result, "expected user input to be 'golang'")
}

func TestTerm_Choose(t *testing.T) {
	t.Parallel()
	NoColorEnv()
	question := "Enter your favorite language: "
	input := "golang\n"
	mockInput := strings.NewReader(input)
	var capturedErr error
	exitFn := func(err error) {
		capturedErr = err
	}
	term := New(WithReader(mockInput), WithInterruptFn(exitFn))
	result, err := term.Choose(question, []string{"golang", "python", "javascript"}, "python")
	assert.NoError(t, err, "expected no error during input")
	assert.NoError(t, capturedErr, "expected no error during input")
	assert.Equal(t, "golang", result, "expected user input to be 'golang'")
}

func TestTermConfirm(t *testing.T) {
	t.Run("confirm valid", func(t *testing.T) {
		t.Parallel()
		question := "Are you sure? "
		term := New(WithReader(strings.NewReader("y\n")))
		assert.True(t, term.Confirm(question, "y"), "user confirms true")
		assert.False(t, term.Confirm(question, "n"), "user cancel")
	})
	t.Run("confirm with ENTER (default)", func(t *testing.T) {
		t.Parallel()
		question := "Continue? "
		term := New(WithReader(strings.NewReader("\n")))
		assert.True(t, term.Confirm(question, "y"), "user confirms true (default)")
		assert.False(t, term.Confirm(question, "n"), "user confirms false (default)")
	})
	t.Run("confirm with invalid input", func(t *testing.T) {
		t.Parallel()
		term := New(WithReader(strings.NewReader("invalid\n")))
		question := "Continue? "
		assert.False(t, term.Confirm(question, "y"), "user cancel")
	})
}

func TestTestConfirmErr(t *testing.T) {
	t.Run("user cancels", func(t *testing.T) {
		t.Parallel()
		term := New(WithReader(strings.NewReader("n\n")))
		err := term.ConfirmErr("continue?", "y")
		assert.Error(t, err, "user cancel")
		assert.ErrorIs(t, err, ErrActionAborted)
	})
	t.Run("exceed attempts", func(t *testing.T) {
		t.Parallel()
		input := "bad\nalso\nwrong\n"
		term := New(WithReader(strings.NewReader(input)))
		err := term.ConfirmErr("continue?", "y")
		assert.Error(t, err, "exceed attempts")
		assert.ErrorIs(t, err, ErrIncorrectAttempts)
	})
	t.Run("valid input", func(t *testing.T) {
		t.Parallel()
		input := "y\n"
		term := New(WithReader(strings.NewReader(input)))
		err := term.ConfirmErr("continue?", "y")
		assert.NoError(t, err)
	})
}

func TestTermIsPiped(t *testing.T) {
	t.Parallel()
	r, _, _ := os.Pipe()
	tests := []struct {
		name   string
		reader io.Reader
		want   bool
	}{
		{
			name:   "piped input",
			reader: bytes.NewBufferString("some input"),
			want:   true,
		},
		{
			name:   "non-piped input",
			reader: os.Stdin,
			want:   false,
		},
		{
			name:   "piped input using os.Pipe()",
			reader: r,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := New(WithReader(tt.reader))
			assert.Equal(t, tt.want, term.IsPiped(), tt.name)
		})
	}
}
