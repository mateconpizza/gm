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
		slog.Debug("environment variable found", "variable", s)
		return v
	}
	slog.Debug("environment variable not set, using default", "variable", s)
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
	_, err := exec.LookPath(s)
	exists := err == nil
	slog.Debug("checked binary", "binary", s, "exists", exists)
	return exists
}

// Which checks if the command exists in $PATH.
func Which(cmd string) (string, error) {
	slog.Debug("looking up command", "command", cmd)
	path, err := exec.LookPath(cmd)
	if err != nil {
		slog.Debug("command not found", "command", cmd, "error", err)
		return "", ErrSysCmdNotFound
	}
	slog.Debug("command found", "command", cmd, "path", path)
	return path, nil
}

// ExecuteCmd runs a command with the given arguments and returns an error if
// the command fails.
func ExecuteCmd(ctx context.Context, arg ...string) error {
	if len(arg) == 0 {
		slog.Error("cannot execute command: no arguments provided")
		return fmt.Errorf("executing command: %w", ErrNoArguments)
	}

	slog.Debug("executing command", "command", arg[0], "args", arg[1:])

	cmd := exec.CommandContext(ctx, arg[0], arg[1:]...)
	if err := cmd.Run(); err != nil {
		slog.Error("command failed", "command", arg[0], "args", arg[1:], "error", err)
		return fmt.Errorf("running %s: %w", arg[0], err)
	}

	slog.Debug("command completed", "command", arg[0])
	return nil
}

// RunCmd returns an *exec.Cmd with the given arguments.
func RunCmd(ctx context.Context, s string, arg ...string) error {
	slog.Debug("running command", "command", s, "args", arg)

	cmd := exec.CommandContext(ctx, s, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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
	args := OSArgs()
	args = append(args, s)
	slog.Debug("opening URL in browser", "url", s)
	return ExecuteCmd(ctx, args...)
}

// CopyClipboard copies a string to the clipboard.
func CopyClipboard(s string) error {
	if err := clipboard.WriteAll(s); err != nil {
		slog.Error("failed to copy text to clipboard", "error", err)
		return fmt.Errorf("%w: %w", ErrCopyToClipboard, err)
	}

	time.Sleep(150 * time.Millisecond)

	slog.Debug("text copied to clipboard", "length", len(s))

	return nil
}

// ReadClipboard reads the contents of the clipboard.
func ReadClipboard() string {
	slog.Debug("reading clipboard")

	s, err := clipboard.ReadAll()
	if err != nil {
		slog.Warn("failed to read clipboard", "error", err)
		return ""
	}

	slog.Debug("clipboard read successfully", "length", len(s))
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
		select {
		case <-ctx.Done():
			slog.Debug("signal context canceled", "cause", context.Cause(ctx))
			return
		case s := <-signals:
			cause := fmt.Errorf("%w with signal %s", err, s)
			slog.Debug("received termination signal",
				"signal", s,
				"cause", cause,
			)
			fmt.Fprintln(os.Stdout)
			cancelCause(cause)
		}
	}()

	return ctx, func() {
		slog.Debug("canceling signal context")
		signal.Stop(signals)
		cancelCause(nil)
		slog.Debug("signal context canceled")
	}
}
