package browser

import (
	"errors"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/terminal"
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
	Import(t *terminal.Term, force bool) (*slice.Slice[bookmark.Bookmark], error)
}
