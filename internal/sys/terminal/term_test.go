//nolint:wsl //test
package terminal

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

//nolint:paralleltest //test
func TestHelper(t *testing.T) {
	t.Helper()
	NoColorEnv()
}

func TestTermPrompt(t *testing.T) {
	t.Parallel()
	question := "Enter your favorite language: "
	want := "golang"
	input := want + "\n"
	mockInput := strings.NewReader(input)
	term := New(WithReader(mockInput))
	got := term.Prompt(question)
	if want != got {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTermChoose(t *testing.T) {
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
	if err != nil {
		t.Errorf("expected no error during input, got: %v", err)
	}

	if capturedErr != nil {
		t.Errorf("expected no captured error, got: %v", capturedErr)
	}

	if result != "golang" {
		t.Errorf("expected user input to be 'golang', got: %q", result)
	}
}

func TestTermConfirm(t *testing.T) {
	t.Run("confirm valid", func(t *testing.T) {
		t.Parallel()
		question := "Are you sure? "
		term := New(WithReader(strings.NewReader("y\n")))
		if !term.Confirm(question, "y") {
			t.Errorf("expected confirmation to be true")
		}
		if term.Confirm(question, "n") {
			t.Errorf("expected confirmation to be false")
		}
	})

	t.Run("confirm with ENTER (default)", func(t *testing.T) {
		t.Parallel()
		question := "Continue? "
		term := New(WithReader(strings.NewReader("\n")))
		if !term.Confirm(question, "y") {
			t.Errorf("expected default confirmation to be true")
		}
		if term.Confirm(question, "n") {
			t.Errorf("expected default confirmation to be false")
		}
	})

	t.Run("confirm with invalid input", func(t *testing.T) {
		t.Parallel()
		term := New(WithReader(strings.NewReader("invalid\n")))
		question := "Continue? "
		if term.Confirm(question, "y") {
			t.Errorf("expected confirmation to be false for invalid input")
		}
	})
}

func TestTestConfirmErr(t *testing.T) {
	t.Parallel()

	t.Run("user cancels", func(t *testing.T) {
		t.Parallel()
		term := New(WithReader(strings.NewReader("n\n")))
		err := term.ConfirmErr("continue?", "y")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrActionAborted) {
			t.Errorf("expected ErrActionAborted, got: %v", err)
		}
	})

	t.Run("exceed attempts", func(t *testing.T) {
		t.Parallel()
		input := "bad\nalso\nwrong\n"
		term := New(WithReader(strings.NewReader(input)))
		err := term.ConfirmErr("continue?", "y")
		if err == nil {
			t.Fatal("expected error due to incorrect attempts, got nil")
		}
		if !errors.Is(err, ErrIncorrectAttempts) {
			t.Errorf("expected ErrIncorrectAttempts, got: %v", err)
		}
	})

	t.Run("valid input", func(t *testing.T) {
		t.Parallel()
		term := New(WithReader(strings.NewReader("y\n")))
		err := term.ConfirmErr("continue?", "y")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
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
		{"piped input", bytes.NewBufferString("some input"), true},
		{"non-piped input", os.Stdin, false},
		{"piped using os.Pipe", r, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := New(WithReader(tt.reader))
			got := term.IsPiped()
			if got != tt.want {
				t.Errorf("IsPiped() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInputPassword(t *testing.T) {
	t.Run("valid password input", func(t *testing.T) {
		t.Parallel()
		pwd := "123"
		input := strings.NewReader(pwd + "\n")
		term := New(WithWriter(io.Discard), WithReader(input))
		s, err := term.InputPassword()
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if s != pwd {
			t.Errorf("expected password %q, got %q", pwd, s)
		}
	})

	t.Run("password mismatch", func(t *testing.T) {
		t.Parallel()
		input := strings.NewReader("password1\npassword2\n")
		term := New(WithWriter(io.Discard), WithReader(input))
		s1, err := term.InputPassword()
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		s2, err := term.InputPassword()
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if s1 == s2 {
			t.Errorf("expected passwords to differ, got same value: %q", s1)
		}
	})
}
