package terminal

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"
)

var (
	Color     bool = true
	Piped     bool = false
	MaxWidth  int  = 120
	MinWidth  int  = 80
	MinHeight int  = 15
)

var (
	ErrNotTTY              = errors.New("not a terminal")
	ErrGetTermSize         = errors.New("getting terminal size")
	ErrTermWidthTooSmall   = errors.New("terminal width too small")
	ErrTermHeightTooSmall  = errors.New("terminal height too small")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

func SetColor(b bool) {
	Color = b
}

func SetIsPiped(b bool) {
	Piped = b
}

func LoadMaxWidth() {
	w, _ := getWidth()
	if w == 0 {
		return
	}

	if w < MaxWidth {
		MaxWidth = w
	}
}

// Clear clears the terminal.
func Clear() {
	fmt.Print("\033[H\033[2J")
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
