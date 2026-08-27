// Package editor provides strategies for editing bookmarks through temporary files.
package editor

import (
	"context"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

type EditStrategy interface {
	// Builds the buffer shown in the editor
	BuildBuffer(m *Meta, b *bookmark.Bookmark, idx, total int) ([]byte, error)

	// Parses buffer back into a bookmark
	ParseBuffer(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error)

	// Compares old/new for diff display
	Diff(old, fresh *bookmark.Bookmark) string

	// Strategy type
	FileType() string
}
