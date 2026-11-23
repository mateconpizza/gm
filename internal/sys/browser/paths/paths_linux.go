package browserpath

import (
	"fmt"
	"os"
	"path/filepath"
)

// genGeckoProfilePath generates the file path to the Gecko-based browser's
// profile configuration on Linux.
func genGeckoProfilePath(p string) string {
	root, _ := resolveLinuxHomePath(p)
	return filepath.Join(root, gecko.profiles)
}

// genGeckoBookmarksPath generates the file path to the Gecko-based browser's
// bookmarks database on Linux.
func genGeckoBookmarksPath(p string) string {
	root, _ := resolveLinuxHomePath(p)
	return filepath.Join(root, "%s", gecko.bookmarks)
}

// genBlinkProfilePath generates the file path to the Blink-based browser's
// profile configuration on Linux.
func genBlinkProfilePath(p string) string {
	root, _ := resolveLinuxConfigPath(p)
	return filepath.Join(root, blink.profiles)
}

// genBlinkBookmarksPath generates the file path to the Blink-based browser's
// bookmarks file on Linux.
func genBlinkBookmarksPath(p string) string {
	root, _ := resolveLinuxConfigPath(p)
	return filepath.Join(root, "%s", blink.bookmarks)
}

func resolveLinuxHomePath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, p), nil
}

func resolveLinuxConfigPath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, ".config", p), nil
}
