package util

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
)

var ErrCopyToClipboard = errors.New("copy to clipboard")

// FilterEntries returns a list of backups.
func FilterEntries(name, path string) ([]fs.DirEntry, error) {
	var filtered []fs.DirEntry
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		if strings.Contains(entry.Name(), name) {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

// GetEnv retrieves an environment variable.
func GetEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return def
}

// BinPath returns the path of the binary.
func BinPath(binaryName string) string {
	cmd := exec.Command("which", binaryName)
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
	cmd := exec.Command("which", binaryName)
	err := cmd.Run()

	return err == nil
}

// ParseUniqueStrings returns a slice of unique strings.
func ParseUniqueStrings(input *[]string, sep string) *[]string {
	uniqueItems := make([]string, 0)
	uniqueMap := make(map[string]struct{})

	for _, tags := range *input {
		tagList := strings.Split(tags, sep)
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				uniqueMap[tag] = struct{}{}
			}
		}
	}

	for tag := range uniqueMap {
		uniqueItems = append(uniqueItems, tag)
	}

	return &uniqueItems
}

// TrimElements returns a slice of the first len(elements) - n elements in the
// input slice.
func TrimElements[T any](elements []T, n int) []T {
	var filtered []T
	if len(elements) > n {
		filtered = elements[:len(elements)-n]
	}

	return filtered
}

// ExecuteCmd runs a command with the given arguments and returns an error if
// the command fails.
func ExecuteCmd(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
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
		args = []string{"cmd", "/c", "start"}
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
