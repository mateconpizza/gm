// Package browser defines an interface for interacting with web browsers to
// import bookmarks.
package browser

import (
	"context"
	"errors"

	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrBrowserUnsupported = errors.New("browser unsupported")

type Supported struct {
	Browser Browser
}

func (s Supported) String() string { return s.Browser.String() }

// Browser defines the interface for interacting with web browsers.
type Browser interface {
	Name() string
	Short() string
	LoadPaths() error
	Import(ctx context.Context, c *ui.Console, force bool) ([]*bookmark.Bookmark, error)
	String() string
}
