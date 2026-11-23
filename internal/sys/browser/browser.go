// Package browser defines an interface for interacting with web browsers to
// import bookmarks.
package browser

import (
	"context"
	"errors"
	"log/slog"

	menu "github.com/mateconpizza/go-fzf"

	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrUnsupported = errors.New("browser unsupported")

type Engine string

func (e Engine) String() string {
	return string(e)
}

type Supported struct {
	Browser Browser
}

func (s Supported) String() string {
	return s.Browser.String()
}

// Browser defines the interface for interacting with web browsers.
type Browser interface {
	Name() string
	LoadPaths(l *slog.Logger) error
	Import(ctx context.Context, l *slog.Logger, m *menu.Menu[string], c *ui.Console) ([]*bookmark.Bookmark, error)
	String() string
	Engine() Engine
}

type Profile struct {
	Name      string
	Path      string
	Source    string
	Engine    Engine
	Bookmarks []*bookmark.Bookmark
}

func NewProfile(name, path, src string, engine Engine) *Profile {
	return &Profile{
		Name:      name,
		Path:      path,
		Source:    src,
		Engine:    engine,
		Bookmarks: make([]*bookmark.Bookmark, 0),
	}
}
