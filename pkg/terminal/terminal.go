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

type Terminal struct {
	MaxWidth  int
	MinWidth  int
	MinHeight int
	Color     bool
}

var Defaults = Terminal{
	MaxWidth:  120,
	MinWidth:  80,
	MinHeight: 15,
	Color:     true,
}

// Size returns the terminal size
func Size() (width, height int, err error) {
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

// IsRedirected returns true if the output is redirected
func IsRedirected() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		log.Println("Error getting stdout file info:", err)
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) == 0
}

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
	var result string
	scanner := bufio.NewScanner(bufio.NewReader(r))
	for scanner.Scan() {
		line := scanner.Text()
		result += line + "\n"
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading from pipe:", err)
		return ""
	}
	return result
}

// inputFromPrompt prompts the user for input
func inputFromPrompt(prompt string) string {
	var s string

	fmt.Printf("%s\n  > ", prompt)

	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	return strings.Trim(s, "\n")
}

// handleTerminalSettings sets the terminal settings
func LoadDefaults(colorFlag string) error {
	if IsRedirected() || colorFlag == "never" {
		Defaults.Color = false
	}

	width, height, err := Size()
	if err != nil {
		if errors.Is(err, ErrNotTTY) {
			return nil
		}
		return fmt.Errorf("getting console size: %w", err)
	}

	if width < Defaults.MinWidth {
		return fmt.Errorf("%w: %d. Min: %d", ErrTermWidthTooSmall, width, Defaults.MinWidth)
	}

	if height < Defaults.MinHeight {
		return fmt.Errorf("%w: %d. Min: %d", ErrTermHeightTooSmall, height, Defaults.MinHeight)
	}

	if width < Defaults.MaxWidth {
		Defaults.MaxWidth = width
	}

	return nil
}
