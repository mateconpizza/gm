package browserpath

import (
	"os"
	"path/filepath"
)

// genGeckoProfilePath generates the file path to the Gecko-based browser's
// profile configuration on macOS.
func genGeckoProfilePath(p string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Library", "Application Support", p, gecko.profiles)
}

// genGeckoBookmarksPath generates the file path to the Gecko-based browser's
// bookmarks database on macOS.
func genGeckoBookmarksPath(p string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Library", "Application Support", p, "%s", gecko.bookmarks)
}

// genBlinkBookmarksPath generates the file path to the Blink-based browser's
// bookmarks file on macOS.
func genBlinkBookmarksPath(p string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Library", "Application Support", p, "%s", blink.bookmarks)
}

// genBlinkProfilePath generates the file path to the Blink-based browser's
// profile configuration on macOS.
func genBlinkProfilePath(p string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Library", "Application Support", p, blink.profiles)
}
