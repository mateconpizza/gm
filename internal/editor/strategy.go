// Package editor provides strategies for editing bookmarks through temporary files.
package editor

import (
	"context"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type EditStrategy interface {
	// Builds the buffer shown in the editor
	BuildBuffer(m *Meta, b *bookmark.Bookmark, idx, total int) ([]byte, error)

	// Parses buffer back into a bookmark
	ParseBuffer(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error)

	// Compares old/new for diff display
	Diff(oldB, newB *bookmark.Bookmark) string

	// Saves changes (to repository)
	Save(ctx context.Context, db *db.SQLite, b *bookmark.Bookmark) error

	// Strategy type
	FileType() string
}
