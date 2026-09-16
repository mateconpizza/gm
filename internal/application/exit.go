package application

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/mateconpizza/gm/internal/sys/cleanup"
)

var (
	ErrActionAborted     = errors.New("action aborted")
	ErrExitFailure       = errors.New("exit failure")
	ErrNotImplementedYet = errors.New("not implemented yet")
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

// Exit logs the error and exits the program.
func Exit(err error) {
	cleanup.Run()

	switch {
	case err == nil:
		os.Exit(ExitSuccess)

	case errors.Is(err, ErrExitFailure):
		slog.Debug(ErrExitFailure.Error())
		os.Exit(ExitFailure)

	case errors.Is(err, ErrActionAborted):
		slog.Debug(ErrActionAborted.Error())
		os.Exit(ExitInterrupted)

	default:
		slog.Warn("exit", "error", err)
		fmt.Fprintf(os.Stderr, "%s: %s\n", Name, err)
		os.Exit(ExitFailure)
	}
}
