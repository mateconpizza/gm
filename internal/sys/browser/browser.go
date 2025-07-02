// Package browser defines an interface for interacting with web browsers to
// import bookmarks.
package browser

import (
	"errors"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/ui"
)

var ErrBrowserUnsupported = errors.New("browser unsupported")

// Browser defines the interface for interacting with various web browsers,
// providing methods to retrieve browser information, load browser paths, and
// import bookmarks.
type Browser interface {
	Name() string
	Short() string
	LoadPaths() error
	Color(string) string
	Import(c *ui.Console, force bool) ([]*bookmark.Bookmark, error)
}
