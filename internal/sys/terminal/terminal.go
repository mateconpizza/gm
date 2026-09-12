// Package terminal provides utilities for interacting with the command-line
// terminal.
package terminal

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"golang.org/x/term"

	"github.com/mateconpizza/gm/internal/sys"
)

var (
	ErrNotTTY            = errors.New("not a terminal")
	ErrNotInteractive    = errors.New("not an interactive terminal")
	ErrIncorrectAttempts = errors.New("incorrect attempts")
)

// force is a flag to force the terminal to run in non-interactive mode.
var force bool = false

func NonInteractiveMode(b bool) {
	force = b
}

type loadSizeFunc func() (width int, height int, err error)

type TermSizeOptions struct {
	loadSize loadSizeFunc
}

type TermSizeOptFn func(*TermSizeOptions)

type TermSize struct {
	TermSizeOptions

	width    int
	maxWidth int
	minWidth int
	height   int
}

func NewSize(opts ...TermSizeOptFn) *TermSize {
	s := &TermSize{
		maxWidth: 120,
		minWidth: 80,
		TermSizeOptions: TermSizeOptions{
			loadSize: getSize,
		},
	}

	for _, opt := range opts {
		opt(&s.TermSizeOptions)
	}

	width, height, err := s.loadSize()
	if err != nil {
		slog.Debug("could not determine terminal size", "error", err)
		return s
	}

	s.width = width
	s.height = height

	if width > 0 && width < s.maxWidth {
		s.maxWidth = width
	}

	return s
}

func withLoadSizeFunc(fn loadSizeFunc) TermSizeOptFn {
	return func(o *TermSizeOptions) {
		o.loadSize = fn
	}
}

func (s *TermSize) Height() int   { return s.height }
func (s *TermSize) MaxWidth() int { return s.maxWidth }
func (s *TermSize) MinWidth() int { return s.minWidth }
func (s *TermSize) Width() int    { return s.width }

// NoColorEnv disables color output if the NO_COLOR environment variable is
// set.
func NoColorEnv() bool {
	// https://no-color.org
	const noColorEnv string = "NO_COLOR"
	c := sys.Env(noColorEnv, "")
	slog.Debug("Environment", slog.String("NO_COLOR", c))
	return c != ""
}

// clearTerminal clears the terminal.
func clearTerminal(w io.Writer) {
	fmt.Fprint(w, "\033[H\033[2J")
}

// ClearChars deletes n characters in the console.
func ClearChars(w io.Writer, n int) {
	for range n {
		fmt.Fprint(w, "\b \b")
	}
}

// ClearLine deletes n lines in the console.
func ClearLine(w io.Writer, n int) {
	for range n {
		fmt.Fprint(w, "\033[F\033[K")
	}
}

// ReplaceLine replaces a line in the console.
func ReplaceLine(w io.Writer, n int, s string) {
	ClearLine(w, n)
	fmt.Fprintln(w, s)
}

// StdinPiped returns true if the input is piped.
func StdinPiped() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) == 0
}

// StdoutPiped reports whether standard output is redirected or piped.
func StdoutPiped() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func getSize() (width, height int, err error) {
	fd := int(os.Stdout.Fd())

	if !term.IsTerminal(fd) {
		return width, height, ErrNotTTY
	}

	width, height, err = term.GetSize(fd)
	if err != nil {
		return width, height, fmt.Errorf("getting console size: %w", err)
	}

	return width, height, nil
}
