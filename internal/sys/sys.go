// Package sys provides system-level utilities for command execution,
// environment interaction, and clipboard operations.
package sys

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
	"github.com/pkg/browser"

	"github.com/mateconpizza/gm/internal/application"
)

var (
	ErrCopyToClipboard   = errors.New("copy to clipboard")
	ErrNotImplementedYet = errors.New("not implemented yet")
	ErrActionAborted     = errors.New("action aborted")
	ErrExitFailure       = errors.New("exit failure")
	ErrSysCmdNotFound    = errors.New("command not found")
)

// Exit codes used by the application.
const (
	// ExitSuccess indicates normal termination.
	ExitSuccess = 0

	// ExitInterrupted is the conventional exit code for Ctrl+C (SIGINT).
	ExitInterrupted = 130

	// ExitFailure indicates a general failure or unhandled error.
	ExitFailure = 1
)

// Env retrieves an environment variable.
//
// If the environment variable is not set, returns the default value.
func Env(s, def string) string {
	if v, ok := os.LookupEnv(s); ok {
		return v
	}

	return def
}

// BinPath returns the path of the binary.
func BinPath(s string) string {
	path, err := exec.LookPath(s)
	if err != nil {
		slog.Debug("binary not found", "which", s, "error", err)
		return ""
	}

	slog.Debug("binary path", "which", s, "found", path)
	return path
}

// BinExists checks if the binary exists in $PATH.
func BinExists(s string) bool {
	return BinPath(s) != ""
}

// Which checks if the command exists in $PATH.
func Which(cmd string) (string, error) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return "", ErrSysCmdNotFound
	}
	return path, nil
}

// ExecuteCmd runs a command with the given arguments and returns an error if
// the command fails.
func ExecuteCmd(ctx context.Context, arg ...string) error {
	cmd := exec.CommandContext(ctx, arg[0], arg[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running command: %w", err)
	}

	return nil
}

// ExecCmdWithWriter runs a command with the given arguments and writes the
// output to the writer.
func ExecCmdWithWriter(ctx context.Context, w io.Writer, s ...string) error {
	slog.Debug("ExecCmdWithWriter", "cmds", s)
	cmd := exec.CommandContext(ctx, s[0], s[1:]...)
	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// RunCmd returns an *exec.Cmd with the given arguments.
func RunCmd(ctx context.Context, s string, arg ...string) error {
	cmd := exec.CommandContext(ctx, s, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	slog.Debug("running command", "command", s, "args", arg)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// OSArgs returns the correct arguments for the OS.
func OSArgs() []string {
	var args []string

	switch runtime.GOOS {
	case "darwin":
		args = []string{"open"}
	case "windows":
		args = []string{"cmd", "/C", "start"}
	default:
		args = []string{"xdg-open"}
	}

	return args
}

// OpenInBrowser opens a URL in the default browser.
func OpenInBrowser(s string) error {
	if err := browser.OpenURL(s); err != nil {
		return fmt.Errorf("%w: opening in browser", err)
	}

	return nil
}

// CopyClipboard copies a string to the clipboard.
func CopyClipboard(s string) error {
	err := clipboard.WriteAll(s)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCopyToClipboard, err)
	}

	time.Sleep(150 * time.Millisecond)

	slog.Debug("text copied to clipboard", "text", s)

	return nil
}

// ReadClipboard reads the contents of the clipboard.
func ReadClipboard() string {
	s, err := clipboard.ReadAll()
	if err != nil {
		slog.Warn("could not read clipboard", "err", err)
		return ""
	}

	return s
}

// ErrAndExit logs the error and exits the program.
func ErrAndExit(err error) {
	switch {
	case err == nil:
		os.Exit(ExitSuccess)

	case errors.Is(err, ErrExitFailure):
		os.Exit(ExitFailure)

	case errors.Is(err, ErrActionAborted):
		slog.Debug("interrupted by user")
		os.Exit(ExitInterrupted)

	default:
		slog.Warn("exit", "error", err)
		fmt.Fprintf(os.Stderr, "%s: %s\n", application.Name, err)
		os.Exit(ExitFailure)
	}
}

// WithSignalContext returns a context that is canceled when an interrupt or
// termination signal is received.
func WithSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancelCause := context.WithCancelCause(parent)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals,
		os.Interrupt,    // Ctrl+C (SIGINT)
		syscall.SIGTERM, // Process termination
		syscall.SIGHUP,  // Terminal closed
	)

	go func() {
		select {
		case <-ctx.Done():
			return // parent canceled
		case s := <-signals:
			slog.Debug("received signal", "signal", s)
			fmt.Println()
			cancelCause(fmt.Errorf("%w with signal %s", ErrActionAborted, s))
		}
	}()

	return ctx, func() {
		signal.Stop(signals)
		cancelCause(nil) // normal cancel
	}
}
