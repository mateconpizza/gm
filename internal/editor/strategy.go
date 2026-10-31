// Package editor provides strategies for editing bookmarks through temporary files.
package editor

import (
	"context"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var Supported = []string{
	"json", "yaml", "bookmark", "notes",
}

type (
	EditStrategy interface {
		// Builds the buffer shown in the editor
		BuildBuffer(b *bookmark.Bookmark, idx, total int) ([]byte, error)

		// Parses buffer back into a bookmark
		ParseBuffer(ctx context.Context, buf []byte, old *bookmark.Bookmark) (*bookmark.Bookmark, error)

		// Compares old/new for diff display
		Diff(d Differ, old, fresh *bookmark.Bookmark) string

		// Strategy type
		FileType() string
	}

	Differ interface {
		Added(s string) string
		Deleted(s string) string
		Muted(s string) string
	}
)
