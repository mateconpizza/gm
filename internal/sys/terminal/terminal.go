package terminal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/term"

	"github.com/mateconpizza/gm/internal/sys"
)

var (
	ErrNotTTY              = errors.New("not a terminal")
	ErrGetTermSize         = errors.New("getting terminal size")
	ErrTermWidthTooSmall   = errors.New("terminal width too small")
	ErrTermHeightTooSmall  = errors.New("terminal height too small")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrNoStateToRestore    = errors.New("no term state to restore")
	ErrNotInteractive      = errors.New("not an interactive terminal")
	ErrIncorrectAttempts   = errors.New("incorrect attempts")
	ErrActionAborted       = errors.New("action aborted")
	ErrCannotBeEmpty       = errors.New("cannot be empty")
)

// termState contains the state of the terminal.
var termState *term.State

// https://no-color.org
const noColorEnv string = "NO_COLOR"

// force is a flag to force the terminal to run in non-interactive mode.
var force bool = false

// Default terminal settings.
var (
	MaxWidth  int = 120
	MinHeight int = 15
	MinWidth  int = 80
)

// NoColorEnv disables color output if the NO_COLOR environment variable is
// set.
func NoColorEnv() bool {
	if c := sys.Env(noColorEnv, ""); c != "" {
		slog.Debug("NO_COLOR", "env found", c)

		return true
	}

	return false
}

// saveState the current terminal state.
func saveState() error {
	slog.Debug("saving terminal state")

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		slog.Debug("not a terminal, skipping saveState")
		return nil
	}

	oldState, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	termState = oldState

	return nil
}

// restoreState the previously saved terminal state.
func restoreState() error {
	slog.Debug("restoring terminal state")

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		slog.Debug("not a terminal, skipping restoreState")
		return nil
	}

	if termState == nil {
		return ErrNoStateToRestore
	}

	if err := term.Restore(int(os.Stdin.Fd()), termState); err != nil {
		return fmt.Errorf("restoring state: %w", err)
	}

	return nil
}

// loadMaxWidth updates `MaxWidth` to the current width if it is smaller than
// the existing `MaxWidth`.
func loadMaxWidth() {
	w, _ := getWidth()
	if w == 0 {
		return
	}

	if w < MaxWidth {
		// MinWidth = w
		MaxWidth = w
	}
}

// clearTerminal clears the terminal.
func clearTerminal() {
	fmt.Print("\033[H\033[2J")
}

// ClearChars deletes n characters in the console.
func ClearChars(n int) {
	for range n {
		fmt.Print("\b \b")
	}
}

// ClearLine deletes n lines in the console.
func ClearLine(n int) {
	for range n {
		fmt.Print("\033[F\033[K")
	}
}

// ReplaceLine replaces a line in the console.
func ReplaceLine(n int, s string) {
	ClearLine(n)
	fmt.Println(s)
}

// IsPiped returns true if the input is piped.
func IsPiped() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) == 0
}

// getWidth returns the terminal's width.
func getWidth() (int, error) {
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return 0, ErrNotTTY
	}

	w, _, err := term.GetSize(fd)
	if err != nil {
		return 0, fmt.Errorf("getting console width: %w", err)
	}

	return w, nil
}

func init() {
	// Loads the terminal settings.
	loadMaxWidth()
}

func NonInteractiveMode(b bool) {
	force = b
}
