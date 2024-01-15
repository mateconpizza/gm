package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/term"
)

var (
	ErrNotTTY              = errors.New("not a terminal")
	ErrGetTermSize         = errors.New("getting terminal size")
	ErrTermWidthTooSmall   = errors.New("terminal width too small")
	ErrTermHeightTooSmall  = errors.New("terminal height too small")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

type TermDefaultSettings struct {
	MaxWidth  int
	MinWidth  int
	MinHeight int
	Color     bool
}

// Settings represents the terminal settings
var Settings = TermDefaultSettings{
	MaxWidth:  120,
	MinWidth:  80,
	MinHeight: 15,
	Color:     true,
}

// dimensions returns the terminal dimensions
func dimensions() (width, height int, err error) {
	fileDescriptor := int(os.Stdout.Fd())

	if !term.IsTerminal(fileDescriptor) {
		return 0, 0, ErrNotTTY
	}

	width, height, err = term.GetSize(fileDescriptor)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrGetTermSize, err)
	}

	return width, height, nil
}

// Clean cleans the terminal
func Clean(msg string) {
	fmt.Print("\033[H\033[2J")
	fmt.Println(msg)
}

// isRedirected returns true if the output is redirected
func isRedirected() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		log.Println("Error getting stdout file info:", err)
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) == 0
}

// IsPiped returns true if the input is piped
func IsPiped() bool {
	fileInfo, _ := os.Stdin.Stat()
	return fileInfo.Mode()&os.ModeCharDevice == 0
}

// ReadInputFromPipe reads the input from the pipe
func ReadInputFromPipe(args *[]string) {
	if !IsPiped() {
		return
	}

	s := getQueryFromPipe(os.Stdin)
	if s == "" {
		return
	}

	split := strings.Split(s, " ")
	*args = append(*args, split...)
}

// getQueryFromPipe returns the query from the pipe
func getQueryFromPipe(r io.Reader) string {
	var result strings.Builder
	scanner := bufio.NewScanner(bufio.NewReader(r))

	for scanner.Scan() {
		line := scanner.Text()
		result.WriteString(line)
		result.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error reading from pipe:", err)
		return ""
	}

	return result.String()
}

// InputFromUserPrompt prompts the user for input
func InputFromUserPrompt(prompt string) string {
	var s string

	fmt.Printf("%s\n  > ", prompt)

	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	return strings.Trim(s, "\n")
}

// LoadDefaults sets the terminal settings
func LoadDefaults(colorFlag string) error {
	if isRedirected() || colorFlag == "never" {
		Settings.Color = false
	}

	width, height, err := dimensions()
	if err != nil {
		if errors.Is(err, ErrNotTTY) {
			return nil
		}
		return fmt.Errorf("getting console size: %w", err)
	}

	if width < Settings.MinWidth {
		return fmt.Errorf("%w: %d. Min: %d", ErrTermWidthTooSmall, width, Settings.MinWidth)
	}

	if height < Settings.MinHeight {
		return fmt.Errorf("%w: %d. Min: %d", ErrTermHeightTooSmall, height, Settings.MinHeight)
	}

	if width < Settings.MaxWidth {
		Settings.MaxWidth = width
	}

	return nil
}
