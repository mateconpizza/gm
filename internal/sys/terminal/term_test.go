package terminal

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelper(t *testing.T) {
	t.Parallel()
	t.Helper()
	NoColorEnv()
}

func TestTerm_Input(t *testing.T) {
	TestHelper(t)
	t.Skip("Skipping test for now")
	input := "yes\n"
	mockInput := strings.NewReader(input)
	var capturedErr error
	exitFn := func(err error) {
		capturedErr = err
	}
	term := New(WithReader(mockInput), WithInterruptFn(exitFn))
	result := term.Input("enter your favorite language: ")
	assert.NoError(t, capturedErr, "expected no error during input")
	assert.Equal(t, "golang", result, "expected user input to be 'golang'")
}

func TestTerm_Prompt(t *testing.T) {
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
	result := term.Choose(question, []string{"golang", "python", "javascript"}, "python")
	assert.NoError(t, capturedErr, "expected no error during input")
	assert.Equal(t, "golang", result, "expected user input to be 'golang'")
}

func TestTerm_Confirm(t *testing.T) {
	t.Parallel()
	question := "Are you sure? "
	yesInput := "y\n"
	term := New()
	mockInput := strings.NewReader(yesInput)
	term.SetReader(mockInput)
	assert.True(t, term.Confirm(question, "y"), "user confirms true")
	assert.False(t, term.Confirm(question, "n"), "user cancel")

	// confirm with ENTER (default)
	enterInput := "\n"
	mockInput = strings.NewReader(enterInput)
	term.SetReader(mockInput)
	assert.True(t, term.Confirm(question, "y"), "user confirms true (default)")
	assert.False(t, term.Confirm(question, "n"), "user confirms false (default)")
}

func TestTerm_IsPiped(t *testing.T) {
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
			term := New(WithReader(tt.reader))
			assert.Equal(t, tt.want, term.IsPiped(), tt.name)
		})
	}
}
