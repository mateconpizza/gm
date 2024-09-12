package terminal

import (
	"errors"
	"fmt"
	"log"
	"os"

	"golang.org/x/term"

	"github.com/haaag/gm/internal/util"
)

// https://no-color.org
const NoColorEnv string = "NO_COLOR"

// Default terminal settings.
var (
	Color     bool = true
	MaxWidth  int  = 120
	MinHeight int  = 15
	MinWidth  int  = 80
	Piped     bool = false
)

var (
	ErrNotTTY              = errors.New("not a terminal")
	ErrGetTermSize         = errors.New("getting terminal size")
	ErrTermWidthTooSmall   = errors.New("terminal width too small")
	ErrTermHeightTooSmall  = errors.New("terminal height too small")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

// SetColor enables or disables color output based on the `NO_COLOR` environment
// variable or a given boolean flag.
func SetColor(b bool) {
	if c := util.GetEnv(NoColorEnv, ""); c != "" {
		log.Println("NO_COLOR found, disabling color output.")
		Color = false
		return
	}

	log.Println("Setting color output:", b)
	Color = b
}

// LoadMaxWidth updates `MaxWidth` to the current width if it is smaller than
// the existing `MaxWidth`.
func LoadMaxWidth() {
	w, _ := getWidth()
	if w == 0 {
		return
	}

	if w < MaxWidth {
		MaxWidth = w
		MinWidth = w
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
