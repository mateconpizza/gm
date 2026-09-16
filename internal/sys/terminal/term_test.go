package terminal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"golang.org/x/term"

	"github.com/mateconpizza/gm/internal/application"
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
	te := New(WithReader(mockInput))
	got, err := te.Prompt(t.Context(), question)
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

	te := New(WithReader(mockInput), WithInterruptFn(exitFn))
	result, err := te.Choose(t.Context(), question, []string{"golang", "python", "javascript"}, "python")
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
		te := New(WithReader(strings.NewReader("y\n")))
		if !te.Confirm(t.Context(), question, "y") {
			t.Errorf("expected confirmation to be true")
		}
		if te.Confirm(t.Context(), question, "n") {
			t.Errorf("expected confirmation to be false")
		}
	})

	t.Run("confirm with ENTER (default)", func(t *testing.T) {
		t.Parallel()
		question := "Continue? "
		te := New(WithReader(strings.NewReader("\n")))
		if !te.Confirm(t.Context(), question, "y") {
			t.Errorf("expected default confirmation to be true")
		}
		if te.Confirm(t.Context(), question, "n") {
			t.Errorf("expected default confirmation to be false")
		}
	})

	t.Run("confirm with invalid input", func(t *testing.T) {
		t.Parallel()
		te := New(WithReader(strings.NewReader("invalid\n")))
		question := "Continue? "
		if te.Confirm(t.Context(), question, "y") {
			t.Errorf("expected confirmation to be false for invalid input")
		}
	})
}

func TestTestConfirmErr(t *testing.T) {
	t.Parallel()

	t.Run("user cancels", func(t *testing.T) {
		t.Parallel()
		te := New(WithReader(strings.NewReader("n\n")))
		err := te.ConfirmErr(t.Context(), "continue?", "y")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, application.ErrExitFailure) {
			t.Errorf("expected ErrActionAborted, got: %v", err)
		}
	})

	t.Run("exceed attempts", func(t *testing.T) {
		t.Parallel()
		input := "bad\nalso\nwrong\n"
		te := New(WithReader(strings.NewReader(input)))
		err := te.ConfirmErr(t.Context(), "continue?", "y")
		if err == nil {
			t.Fatal("expected error due to incorrect attempts, got nil")
		}
		if !errors.Is(err, ErrIncorrectAttempts) {
			t.Errorf("expected ErrIncorrectAttempts, got: %v", err)
		}
	})

	t.Run("valid input", func(t *testing.T) {
		t.Parallel()
		te := New(WithReader(strings.NewReader("y\n")))
		err := te.ConfirmErr(t.Context(), "continue?", "y")
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
			te := New(WithReader(tt.reader))
			got := te.StdinPiped()
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
		te := New(WithWriter(io.Discard), WithReader(input))
		s, err := te.InputPassword(t.Context())
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
		te := New(WithWriter(io.Discard), WithReader(input))
		s1, err := te.InputPassword(t.Context())
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		s2, err := te.InputPassword(t.Context())
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
			wantErr:      application.ErrActionAborted,
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

func TestTerm_paginate(t *testing.T) {
	tests := []struct {
		name       string
		pagerEnv   string // "" and unset are different cases below
		envUnset   bool
		runErr     error
		wantRunCmd []string // expected args passed to pagerRun; nil = pagerRun should not be called
		wantOutput string   // content written to t.writer
	}{
		{
			name:       "pager_explicitly_disabled",
			pagerEnv:   "",
			wantOutput: "hello world",
		},
		{
			name:       "default_pager_when_unset",
			envUnset:   true,
			wantRunCmd: []string{"less", "-RFX"},
		},
		{
			name:       "custom_pager_with_args",
			pagerEnv:   "bat --paging=always",
			wantRunCmd: []string{"bat", "--paging=always"},
		},
		{
			name:       "pager_run_fails_falls_back_to_writer",
			pagerEnv:   "less",
			runErr:     errors.New("exit status 1"),
			wantRunCmd: []string{"less"},
			wantOutput: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envUnset {
				t.Setenv("PAGER", "")
				os.Unsetenv("PAGER") // t.Setenv alone can't represent "unset"; belt and suspenders
			} else {
				t.Setenv("PAGER", tt.pagerEnv)
			}

			var gotArgs []string
			var buf bytes.Buffer

			te := New(WithWriter(&buf))
			te.pagerFunc = func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
				gotArgs = args
				return tt.runErr
			}

			err := te.paginate(t.Context(), "hello world")
			if err != nil {
				t.Fatalf("paginate() unexpected error: %v", err)
			}

			if !slices.Equal(gotArgs, tt.wantRunCmd) {
				t.Errorf("pagerRun args = %v, want %v", gotArgs, tt.wantRunCmd)
			}
			if buf.String() != tt.wantOutput {
				t.Errorf("writer content = %q, want %q", buf.String(), tt.wantOutput)
			}
		})
	}
}

func TestNewSize(t *testing.T) {
	t.Parallel()

	errMock := errors.New("mock error")

	tests := []struct {
		name        string
		getSizeFunc func() (width int, height int, err error)
		wantWidth   int
		wantHeight  int
		wantMaxW    int
		wantMinW    int
	}{
		{
			name: "normal_typical_size",
			getSizeFunc: func() (width int, height int, err error) {
				return 100, 30, nil
			},
			wantWidth:  100,
			wantHeight: 30,
			wantMaxW:   100, // 100 > 0 && 100 < 120, so maxWidth becomes 100
			wantMinW:   80,
		},
		{
			name: "error_from_getsize",
			getSizeFunc: func() (width int, height int, err error) {
				return 0, 0, errMock
			},
			wantWidth:  0,
			wantHeight: 0,
			wantMaxW:   120, // default
			wantMinW:   80,  // default
		},
		{
			name: "zero_dimensions",
			getSizeFunc: func() (width int, height int, err error) {
				return 0, 0, nil
			},
			wantWidth:  0,
			wantHeight: 0,
			wantMaxW:   120, // 0 is not > 0, so maxWidth remains default
			wantMinW:   80,
		},
		{
			name: "width_exact_upper_bound",
			getSizeFunc: func() (width int, height int, err error) {
				return 120, 40, nil
			},
			wantWidth:  120,
			wantHeight: 40,
			wantMaxW:   120, // 120 is not < 120, so maxWidth remains default
			wantMinW:   80,
		},
		{
			name: "width_above_upper_bound",
			getSizeFunc: func() (width int, height int, err error) {
				return 150, 40, nil
			},
			wantWidth:  150,
			wantHeight: 40,
			wantMaxW:   120, // 150 is not < 120, so maxWidth remains default
			wantMinW:   80,
		},
		{
			name: "width_just_below_upper_bound",
			getSizeFunc: func() (width int, height int, err error) {
				return 119, 40, nil
			},
			wantWidth:  119,
			wantHeight: 40,
			wantMaxW:   119, // 119 > 0 && 119 < 120, so maxWidth becomes 119
			wantMinW:   80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewSize(withLoadSizeFunc(tt.getSizeFunc))
			if got.Width() != tt.wantWidth {
				t.Errorf("Width() = %d, want %d", got.Width(), tt.wantWidth)
			}
			if got.Height() != tt.wantHeight {
				t.Errorf("Height() = %d, want %d", got.Height(), tt.wantHeight)
			}
			if got.MaxWidth() != tt.wantMaxW {
				t.Errorf("MaxWidth() = %d, want %d", got.MaxWidth(), tt.wantMaxW)
			}
			if got.MinWidth() != tt.wantMinW {
				t.Errorf("MinWidth() = %d, want %d", got.MinWidth(), tt.wantMinW)
			}
		})
	}
}
