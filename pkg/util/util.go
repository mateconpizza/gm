package util

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

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

// Spinner displays a spinning cursor animation while waiting for a signal on a
// channel.
func Spinner(done chan bool, mesg string) {
	spinner := []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇"}
	for i := 0; ; i++ {
		select {
		case <-done:
			fmt.Printf("\r%-*s\r", len(mesg)+2, " ")
			return
		default:
			fmt.Printf("\r%s %s", spinner[i%len(spinner)], mesg)
			time.Sleep(110 * time.Millisecond)
		}
	}
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

// TrimElements returns a slice of the first len(elements) - n
// elements in the input slice.
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
