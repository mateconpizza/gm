package browserpath

import (
	"os"
	"path/filepath"
)

// genGeckoProfilePath generates the file path to the Gecko-based browser's
// profile configuration.
func genGeckoProfilePath(p string) string {
	appData := os.Getenv("APPDATA")
	return filepath.Join(appData, p, "profiles.ini")
}

// genGeckoBookmarksPath generates the file path to the Gecko-based browser's
// bookmarks database.
func genGeckoBookmarksPath(p string) string {
	appData := os.Getenv("APPDATA")
	return filepath.Join(appData, p, "%s", "places.sqlite")
}

// genBlinkProfilePath generates the file path to the Blink-based browser's
// profile configuration.
func genBlinkProfilePath(p string) string {
	localAppData := os.Getenv("LOCALAPPDATA")
	return filepath.Join(localAppData, p, "Local State")
}

// genBlinkBookmarksPath generates the file path to the Blink-based browser's
// bookmarks file.
func genBlinkBookmarksPath(p string) string {
	localAppData := os.Getenv("LOCALAPPDATA")
	return filepath.Join(localAppData, p, "%s", "Bookmarks")
}
