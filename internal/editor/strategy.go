// Package editor provides strategies for editing bookmarks through temporary files.
package editor

import (
	"context"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

type EditStrategy interface {
	// Builds the buffer shown in the editor
	BuildBuffer(dbName, version string, b *bookmark.Bookmark, idx, total int) ([]byte, error)

	// Parses buffer back into a bookmark
	ParseBuffer(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error)

	// Compares old/new for diff display
	Diff(d Differ, old, fresh *bookmark.Bookmark) string

	// Strategy type
	FileType() string
}

type Differ interface {
	Added(s string) string
	Deleted(s string) string
	Muted(s string) string
}
