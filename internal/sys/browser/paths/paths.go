// Package browserpath provides functions to generate platform-specific paths
// for browser profiles and bookmark files.
package browserpath

type BrowserFiles struct {
	profiles  string
	bookmarks string
}

var (
	gecko = BrowserFiles{
		profiles:  "profiles.ini",
		bookmarks: "places.sqlite",
	}

	blink = BrowserFiles{
		profiles:  "Local State",
		bookmarks: "Bookmarks",
	}
)

// GeckoBookmarkPath returns the path to the Gecko-based browser's bookmarks
// file.
func GeckoBookmarkPath(p string) string {
	return genGeckoBookmarksPath(p)
}

// GeckoProfilePath returns the path to the Gecko-based browser's profile file.
func GeckoProfilePath(p string) string {
	return genGeckoProfilePath(p)
}

// BlinkProfilePath returns the path to the Blink-based browser's profile file.
func BlinkProfilePath(p string) string {
	return genBlinkProfilePath(p)
}

// BlinkBookmarksPath returns the path to the Blink-based browser's bookmarks
// file.
func BlinkBookmarksPath(p string) string {
	return genBlinkBookmarksPath(p)
}
