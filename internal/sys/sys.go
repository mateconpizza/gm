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
)

var (
	ErrCopyToClipboard   = errors.New("copy to clipboard")
	ErrNotImplementedYet = errors.New("not implemented yet")
)

// GetEnv retrieves an environment variable.
//
// If the environment variable is not set, returns the default value.
func GetEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return def
}

// BinPath returns the path of the binary.
func BinPath(binaryName string) string {
	cmd := exec.CommandContext(context.Background(), "which", binaryName)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	c := strings.TrimRight(string(out), "\n")
	log.Printf("which %s = %s", binaryName, c)

	return c
}

// BinExists checks if the binary exists in $PATH.
func BinExists(binaryName string) bool {
	return ExecuteCmd("which", binaryName) == nil
}

// ExecuteCmd runs a command with the given arguments and returns an error if
// the command fails.
func ExecuteCmd(args ...string) error {
	cmd := exec.CommandContext(context.Background(), args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running command: %w", err)
	}

	return nil
}

// RunCmd returns an *exec.Cmd with the given arguments.
func RunCmd(name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("running command: %w", err)
	}

	return nil
}

// GetOSArgsCmd returns the correct arguments for the OS.
func GetOSArgsCmd() []string {
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
func OpenInBrowser(url string) error {
	args := append(GetOSArgsCmd(), url)
	if err := ExecuteCmd(args...); err != nil {
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
