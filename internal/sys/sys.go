package sys

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/pkg/browser"
)

var (
	ErrCopyToClipboard   = errors.New("copy to clipboard")
	ErrNotImplementedYet = errors.New("not implemented yet")
)

// Env retrieves an environment variable.
//
// If the environment variable is not set, returns the default value.
func Env(s, def string) string {
	if v, ok := os.LookupEnv(s); ok {
		return v
	}

	return def
}

// BinPath returns the path of the binary.
func BinPath(s string) string {
	cmd := exec.CommandContext(context.Background(), "which", s)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	c := strings.TrimRight(string(out), "\n")
	log.Printf("which %s = %s", s, c)

	return c
}

// BinExists checks if the binary exists in $PATH.
func BinExists(s string) bool {
	return ExecuteCmd("which", s) == nil
}

// ExecuteCmd runs a command with the given arguments and returns an error if
// the command fails.
func ExecuteCmd(arg ...string) error {
	cmd := exec.CommandContext(context.Background(), arg[0], arg[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running command: %w", err)
	}

	return nil
}

// RunCmd returns an *exec.Cmd with the given arguments.
func RunCmd(s string, arg ...string) error {
	cmd := exec.CommandContext(context.Background(), s, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("running command: %w", err)
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

	return args
}

// OpenInBrowser opens a URL in the default browser.
func OpenInBrowser(s string) error {
	if err := browser.OpenURL(s); err != nil {
		return fmt.Errorf("%w: opening in browser", err)
	}

	return nil
}

// CopyClipboard copies a string to the clipboard.
func CopyClipboard(s string) error {
	err := clipboard.WriteAll(s)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCopyToClipboard, err)
	}

	log.Print("text copied to clipboard:", s)

	return nil
}

// ReadClipboard reads the contents of the clipboard.
func ReadClipboard() string {
	s, err := clipboard.ReadAll()
	if err != nil {
		log.Printf("could not read clipboard: %s\n", err)
		return ""
	}

	return s
}
