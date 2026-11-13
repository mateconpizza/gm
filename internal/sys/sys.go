// Package sys provides system-level utilities for command execution,
// environment interaction, and clipboard operations.
package sys

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
)

var (
	ErrCopyToClipboard = errors.New("copy to clipboard")
	ErrSysCmdNotFound  = errors.New("command not found")
	ErrNoArguments     = errors.New("no arguments provided")
)

// Env retrieves an environment variable.
//
// If the environment variable is not set, returns the default value.
func Env(s, def string) string {
	if v, ok := os.LookupEnv(s); ok {
		slog.Debug("environment variable set", "fn", "env", "variable", s)
		return v
	}

	return def
}

// BinPath returns the path of the binary.
func BinPath(s string) string {
	path, err := exec.LookPath(s)
	if err != nil {
		return ""
	}

	return path
}

// BinExists checks if the binary exists in $PATH.
func BinExists(s string) bool {
	_, err := exec.LookPath(s)
	return err == nil
}

// Which checks if the command exists in $PATH.
func Which(cmd string) (string, error) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		slog.Debug("command not found", "fn", "which", "command", cmd, "error", err)
		return "", fmt.Errorf("%w: %q", ErrSysCmdNotFound, cmd)
	}

	return path, nil
}

// ExecuteCmd runs a command with the given arguments and returns an error if
// the command fails.
func ExecuteCmd(ctx context.Context, arg ...string) error {
	if len(arg) == 0 {
		return fmt.Errorf("executing command: %w", ErrNoArguments)
	}

	slog.Debug("executing command", "fn", "execute-cmd", "command", arg[0], "nargs", len(arg)-1)

	cmd := exec.CommandContext(ctx, arg[0], arg[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s: %w", arg[0], err)
	}

	return nil
}

// RunCmd returns an *exec.Cmd with the given arguments.
func RunCmd(ctx context.Context, s string, arg ...string) error {
	slog.Debug("running command", "fn", "run-cmd", "command", s, "nargs", len(arg))

	cmd := exec.CommandContext(ctx, s, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s: %w", s, err)
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
	slog.Debug("selected OS command", "os", runtime.GOOS, "args", args)
	return args
}

// OpenInBrowser opens a URL in the default browser.
func OpenInBrowser(ctx context.Context, s string) error {
	args := append(OSArgs(), s)
	return ExecuteCmd(ctx, args...)
}

// CopyClipboard copies a string to the clipboard.
func CopyClipboard(s string) error {
	if err := clipboard.WriteAll(s); err != nil {
		return fmt.Errorf("%w: %w", ErrCopyToClipboard, err)
	}

	time.Sleep(150 * time.Millisecond)
	slog.Debug("text copied to clipboard", "fn", "copy-clipboard", "length", len(s))

	return nil
}

// ReadClipboard reads the contents of the clipboard.
func ReadClipboard() string {
	s, err := clipboard.ReadAll()
	if err != nil {
		slog.Warn("failed to read clipboard", "fn", "read-clipboard", "error", err)
		return ""
	}

	slog.Debug("clipboard read", "fn", "read-clipboard", "length", len(s))

	return s
}

// WithSignalContext returns a context that is canceled when an interrupt or
// termination signal is received.
func WithSignalContext(parent context.Context, err error) (context.Context, context.CancelFunc) {
	ctx, cancelCause := context.WithCancelCause(parent)

	signals := make(chan os.Signal, 1)
	signal.Notify(
		signals,
		os.Interrupt,    // Ctrl+C (SIGINT)
		syscall.SIGTERM, // Process termination
		syscall.SIGHUP,  // Terminal closed
	)

	slog.Debug("signal context initialized")

	go func() {
		// always release the signal registration when the goroutine exits,
		// whether by signal, parent cancel, or the returned cancel func.
		defer signal.Stop(signals)

		select {
		case <-ctx.Done():
			slog.Debug("context done", "cause", context.Cause(ctx))
			return
		case s := <-signals:
			cause := fmt.Errorf("%w with signal %s", err, s)
			slog.Debug("received termination signal", "signal", s.String(), "cause", cause)
			fmt.Fprintln(os.Stdout)
			cancelCause(cause)
		}
	}()

	return ctx, func() {
		slog.Debug("canceling", "fn", "signal-context")
		cancelCause(nil)
	}
}
