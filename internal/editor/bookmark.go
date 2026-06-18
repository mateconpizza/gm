package editor

import (
	"context"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var _ EditStrategy = (*BookmarkStrategy)(nil)

type BookmarkStrategy struct{}

func (BookmarkStrategy) BuildBuffer(m *Meta, b *bookmark.Bookmark, idx, total int) ([]byte, error) {
	var (
		pad   = 10
		f     = frame.New(frame.WithBordersCustom("# ", "# ", "# ", "# "))
		char  = "-"
		width = terminal.MinWidth()
	)

	isNewBookmark := b.ID == 0
	label := " bookmark edition "
	if isNewBookmark {
		label = " bookmark addition "
	}
	footer := fmt.Sprintf(" [%d/%d]", idx, total)
	if isNewBookmark {
		footer = " [New]"
	}

	separator := txt.SpanCenter(width-2, "", char)
	idTitleLine := fmt.Sprintf("%d %s", b.ID, txt.Shorten(b.Title, width))
	dbName := txt.PaddedLineWithPad("database:", m.DBName, pad)
	version := txt.PaddedLineWithPad("version:", formatVersion(m.Version), pad)
	sepTitle := txt.SpanCenter(width-2, label, char)

	return f.
		Headerln(separator). // -----------------
		Midln(idTitleLine).  // ID Title
		Rowln().             //
		Midln(dbName).       // database: dbName
		Midln(version).      // version: x.x.x
		Midln(sepTitle).     // ----- label -----
		Ln().
		Text(string(b.Buffer())).
		Text(footer).
		Bytes(), nil
}

func (BookmarkStrategy) ParseBuffer(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
	edited := original.Copy()
	bookmarkFromBytes(buf, edited)
	edited.Notes = original.Notes
	if original.Equals(edited) {
		return nil, ErrBufferUnchanged
	}

	edited = metadata.EnrichBookmark(ctx, edited)
	if err := bookmark.Validate(edited); err != nil {
		return nil, err
	}

	return edited, nil
}

func (BookmarkStrategy) Diff(oldB, newB *bookmark.Bookmark) string {
	return txt.DiffColorize(txt.Diff(oldB.Buffer(), newB.Buffer()))
}

func (BookmarkStrategy) FileType() string { return application.Name }
func (BookmarkStrategy) Save(ctx context.Context, r *db.SQLite, bm *bookmark.Bookmark) error {
	return r.UpdateOne(ctx, bm)
}

func bookmarkFromBytes(buf []byte, b *bookmark.Bookmark) {
	lines := strings.Split(string(buf), "\n") // bytes to lines
	b.URL = txt.CleanLines(txt.ExtractBlock(lines, "# *URL:", "# Title:"))
	b.Title = txt.CleanLines(txt.ExtractBlock(lines, "# Title:", "# Tags:"))
	b.Tags = bookmark.ParseTags(txt.CleanLines(txt.ExtractBlock(lines, "# Tags:", "# Description:")))
	b.Desc = txt.CleanLines(txt.ExtractBlock(lines, "# Description:", "# end"))
}

func NewBookmarkStrategy() *BookmarkStrategy {
	return &BookmarkStrategy{}
}

// formatVersion formats the version string.
func formatVersion(v string) string {
	if v == "dev" {
		return v
	}
	return "v" + v
}
