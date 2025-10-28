// Package editor provides strategies for editing bookmarks through temporary files.
package editor

import (
	"context"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type Record = bookmark.Bookmark

type EditStrategy interface {
	// Builds the buffer shown in the editor
	BuildBuffer(m *Meta, b *Record, idx, total int) ([]byte, error)

	// Parses buffer back into a bookmark
	ParseBuffer(ctx context.Context, buf []byte, original *Record, idx, total int) (*Record, error)

	// Compares old/new for diff display
	Diff(oldB, newB *Record) string

	// Saves changes (to repository)
	Save(ctx context.Context, db *db.SQLite, b *Record) error
}

// Strategy returns the appropriate EditStrategy and filetype
// based on configuration flags.
func Strategy(cfg *config.Config) (strategy EditStrategy, filetype string) {
	switch {
	case cfg.Flags.Notes:
		return NotesStrategy{}, cfg.Name
	case cfg.Flags.Format == "j" || cfg.Flags.Format == "json":
		return JSONStrategy{}, "json"
	case cfg.Flags.Create:
		return NewBookmarkStrategy{}, cfg.Name
	default:
		return BookmarkStrategy{}, cfg.Name
	}
}
