package browserpath

import (
	"fmt"
	"os"
	"path/filepath"
)

// genGeckoProfilePath generates the file path to the Gecko-based browser's
// profile configuration on macOS.
func genGeckoProfilePath(p string) string {
	h, _ := resolveDarwinSupportPath(p)
	return filepath.Join(h, p, gecko.profiles)
}

// genGeckoBookmarksPath generates the file path to the Gecko-based browser's
// bookmarks database on macOS.
func genGeckoBookmarksPath(p string) string {
	h, _ := resolveDarwinSupportPath(p)
	return filepath.Join(h, p, "%s", gecko.bookmarks)
}

// genBlinkBookmarksPath generates the file path to the Blink-based browser's
// bookmarks file on macOS.
func genBlinkBookmarksPath(p string) string {
	h, _ := resolveDarwinSupportPath(p)
	return filepath.Join(h, p, "%s", blink.bookmarks)
}

// genBlinkProfilePath generates the file path to the Blink-based browser's
// profile configuration on macOS.
func genBlinkProfilePath(p string) string {
	h, _ := resolveDarwinSupportPath(p)
	return filepath.Join(h, p, blink.profiles)
}

func resolveDarwinSupportPath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, "Library", "Application Support", p), nil
}


func PavelProfile(p string) string {
	h, _ := resolveDarwinSupportPath(p)
	return filepath.Join(h, p, gecko.profiles)
}

func PavelBookmarks(p string) string {
	h, _ := resolveDarwinSupportPath(p)
	return filepath.Join(h, p, "%s", gecko.bookmarks)
}
