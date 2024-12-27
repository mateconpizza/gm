package browser

import (
	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/slice"
)

// Browser defines the interface for interacting with various web browsers,
// providing methods to retrieve browser information, load browser paths, and
// import bookmarks.
type Browser interface {
	Name() string
	Short() string
	LoadPaths() error
	Color(string) string
	Paths() ([]string, error)
	Import() (*slice.Slice[bookmark.Bookmark], error)
}
