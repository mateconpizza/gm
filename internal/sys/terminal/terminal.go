package terminal

import (
	"errors"
	"fmt"
	"log"
	"os"

	"golang.org/x/term"

	"github.com/haaag/gm/internal/sys"
)

var (
	termState    *term.State
	enabledColor *bool
)

// https://no-color.org
const noColorEnv string = "NO_COLOR"

// Default terminal settings.
var (
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
	ErrNoStateToRestore    = errors.New("no term state to restore")
)

// NoColor disables color output if the NO_COLOR environment variable is set.
func NoColor(b *bool) {
	if c := sys.Env(noColorEnv, ""); c != "" {
		log.Println("'NO_COLOR' environment variable found.")
		*b = false
	}

	enabledColor = b
}

// Save the current terminal state.
func saveState() error {
	oldState, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("saving state: %w", err)
	}
	termState = oldState

	return nil
}

// Restore the previously saved terminal state.
func restoreState() error {
	if termState == nil {
		return ErrNoStateToRestore
	}

	err := term.Restore(int(os.Stdin.Fd()), termState)
	if err != nil {
		return fmt.Errorf("restoring state: %w", err)
	}

	return nil
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
		// MinWidth = w
	}
}

// Clear clears the terminal.
func Clear() {
	fmt.Print("\033[H\033[2J")
}

// ClearChars deletes n characters in the console.
func ClearChars(n int) {
	if n <= 0 || !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	for i := 0; i < n; i++ {
		fmt.Print("\b \b")
	}
}

// ClearLine deletes n lines in the console.
func ClearLine(n int) {
	if n <= 0 || !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	for i := 0; i < n; i++ {
		fmt.Print("\033[F\033[K")
	}
}

func ReplaceLine(n int, s string) {
	if n <= 0 || !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

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
