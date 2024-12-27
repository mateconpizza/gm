package browserpath

import (
	"os"
	"path/filepath"
)

// genGeckoProfilePath generates the file path to the Gecko-based browser's
// profile configuration on Linux.
func genGeckoProfilePath(p string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, p, "profiles.ini")
}

// genGeckoBookmarksPath generates the file path to the Gecko-based browser's
// bookmarks database on Linux.
func genGeckoBookmarksPath(p string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, p, "%s", "places.sqlite")
}

// genBlinkProfilePath generates the file path to the Blink-based browser's
// profile configuration on Linux.
func genBlinkProfilePath(p string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", p, "Local State")
}

// genBlinkBookmarksPath generates the file path to the Blink-based browser's
// bookmarks file on Linux.
func genBlinkBookmarksPath(p string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", p, "%s", "Bookmarks")
}
