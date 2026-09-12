package terminal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"testing/iotest"

	"golang.org/x/term"

	"github.com/mateconpizza/gm/internal/sys"
)

func TestHelper(t *testing.T) {
	t.Parallel()
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
	got, err := term.Prompt(t.Context(), question)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	result, err := term.Choose(t.Context(), question, []string{"golang", "python", "javascript"}, "python")
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
	t.Parallel()

	t.Run("confirm valid", func(t *testing.T) {
		t.Parallel()
		question := "Are you sure? "
		term := New(WithReader(strings.NewReader("y\n")))
		if !term.Confirm(t.Context(), question, "y") {
			t.Errorf("expected confirmation to be true")
		}
		if term.Confirm(t.Context(), question, "n") {
			t.Errorf("expected confirmation to be false")
		}
	})

	t.Run("confirm with ENTER (default)", func(t *testing.T) {
		t.Parallel()
		question := "Continue? "
		term := New(WithReader(strings.NewReader("\n")))
		if !term.Confirm(t.Context(), question, "y") {
			t.Errorf("expected default confirmation to be true")
		}
		if term.Confirm(t.Context(), question, "n") {
			t.Errorf("expected default confirmation to be false")
		}
	})

	t.Run("confirm with invalid input", func(t *testing.T) {
		t.Parallel()
		term := New(WithReader(strings.NewReader("invalid\n")))
		question := "Continue? "
		if term.Confirm(t.Context(), question, "y") {
			t.Errorf("expected confirmation to be false for invalid input")
		}
	})
}

func TestTestConfirmErr(t *testing.T) {
	t.Parallel()

	t.Run("user cancels", func(t *testing.T) {
		t.Parallel()
		term := New(WithReader(strings.NewReader("n\n")))
		err := term.ConfirmErr(t.Context(), "continue?", "y")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, sys.ErrExitFailure) {
			t.Errorf("expected ErrActionAborted, got: %v", err)
		}
	})

	t.Run("exceed attempts", func(t *testing.T) {
		t.Parallel()
		input := "bad\nalso\nwrong\n"
		term := New(WithReader(strings.NewReader(input)))
		err := term.ConfirmErr(t.Context(), "continue?", "y")
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
		err := term.ConfirmErr(t.Context(), "continue?", "y")
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
			got := term.StdinPiped()
			if got != tt.want {
				t.Errorf("IsPiped() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInputPassword(t *testing.T) {
	t.Parallel()

	t.Run("valid password input", func(t *testing.T) {
		t.Parallel()
		pwd := "123"
		input := strings.NewReader(pwd + "\n")
		term := New(WithWriter(io.Discard), WithReader(input))
		s, err := term.InputPassword(t.Context())
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
		s1, err := term.InputPassword(t.Context())
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		s2, err := term.InputPassword(t.Context())
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if s1 == s2 {
			t.Errorf("expected passwords to differ, got same value: %q", s1)
		}
	})
}

func TestTerm_InputPassword(t *testing.T) {
	t.Parallel()

	errRead := errors.New("broken pipe")
	errTermRead := errors.New("read /dev/tty: input/output error")

	tests := []struct {
		name         string
		isTerminal   bool
		input        string // used only when isTerminal == false
		readErr      error  // used only when isTerminal == false
		readPassword string // used only when isTerminal == true
		readPassErr  error  // used only when isTerminal == true
		cancelBefore bool   // cancel ctx before readPassword resolves
		want         string
		wantErr      error
		wantErrMsg   string
	}{
		{
			name:       "normal_password_with_newline",
			isTerminal: false,
			input:      "hunter2\n",
			want:       "hunter2",
		},
		{
			name:       "empty_password_with_newline",
			isTerminal: false,
			input:      "\n",
			want:       "",
		},
		{
			name:       "password_no_trailing_newline_eof",
			isTerminal: false,
			input:      "hunter2",
			want:       "hunter2",
		},
		{
			name:       "reader_error_non_eof",
			isTerminal: false,
			readErr:    errRead,
			wantErrMsg: "reading password",
		},
		{
			name:         "terminal_reads_password_successfully",
			isTerminal:   true,
			readPassword: "hunter2",
			want:         "hunter2",
		},
		{
			name:        "terminal_read_password_fails",
			isTerminal:  true,
			readPassErr: errTermRead,
			wantErrMsg:  "reading password",
		},
		{
			name:         "context_cancelled_before_password_read",
			isTerminal:   true,
			readPassword: "hunter2",
			cancelBefore: true,
			wantErr:      sys.ErrActionAborted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var r *bufio.Reader
			if tt.readErr != nil {
				r = bufio.NewReader(iotest.ErrReader(tt.readErr))
			} else {
				r = bufio.NewReader(strings.NewReader(tt.input))
			}

			blockChan := make(chan struct{})

			state := NewState().
				WithSaveFunc(func() (*term.State, error) { return &term.State{}, nil }).
				WithRestoreFunc(func(state *term.State) error { return nil })

			te := New(WithReader(r), WithTermState(state))
			te.isTerminal = func(fd int) bool { return tt.isTerminal }
			te.readPassword = func(fd int) ([]byte, error) {
				if tt.cancelBefore {
					<-blockChan // block until the test cancels ctx and releases us
				}
				return []byte(tt.readPassword), tt.readPassErr
			}

			ctx := t.Context()
			if tt.cancelBefore {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				close(blockChan) // let the goroutine proceed after cancellation is observed
			}

			got, err := te.InputPassword(ctx)

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("InputPassword() error = %v, want %v", err, tt.wantErr)
				}
			case tt.wantErrMsg != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("InputPassword() error = %v, want containing %q", err, tt.wantErrMsg)
				}
			case err != nil:
				t.Fatalf("InputPassword() unexpected error: %v", err)
			default:
				if got != tt.want {
					t.Errorf("InputPassword() = %q, want %q", got, tt.want)
				}
			}
		})
	}
}
