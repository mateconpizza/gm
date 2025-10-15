// Package editor provides strategies for editing bookmarks through temporary files.
package editor

import (
	"context"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type Record = bookmark.Bookmark

type EditStrategy interface {
	// Builds the buffer shown in the editor
	BuildBuffer(b *Record, idx, total int) ([]byte, error)

	// Parses buffer back into a bookmark
	ParseBuffer(ctx context.Context, buf []byte, original *Record, idx, total int) (*Record, error)

	// Compares old/new for diff display
	Diff(oldB, newB *Record) string

	// Saves changes (to repository)
	Save(ctx context.Context, db *db.SQLite, b *Record) error

	// EditType type for the tempfile
	EditType() string
}
