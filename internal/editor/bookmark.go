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
)

var _ EditStrategy = (*BookmarkStrategy)(nil)

type BookmarkParseBufFunc func(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error)

type BookmarkStrategy struct {
	parseBuffer BookmarkParseBufFunc
}

func NewBookmarkStrategy() *BookmarkStrategy {
	return &BookmarkStrategy{parseBuffer: defaultParseBuffer}
}

func (bs *BookmarkStrategy) WithParseBuffer(fn BookmarkParseBufFunc) *BookmarkStrategy {
	bs.parseBuffer = fn
	return bs
}

func (bs *BookmarkStrategy) BuildBuffer(m *Meta, b *bookmark.Bookmark, idx, total int) ([]byte, error) {
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
	title := strings.ReplaceAll(b.Title, "\n", " ")
	idTitleLine := fmt.Sprintf("%d %s", b.ID, txt.Shorten(title, width-6))
	dbName := txt.PaddedLineWithPad("database:", m.dbName, pad)
	version := txt.PaddedLineWithPad("version:", formatVersion(m.version), pad)
	sepTitle := txt.SpanCenter(width-2, label, char)

	return f.
		Headerln(separator).      // -------------------
		Midln(idTitleLine).       // ID Title
		Rowln().                  //
		Midln(dbName).            // database: dbName
		Midln(version).           // version: x.x.x
		Midln(sepTitle).          // ----- label -------
		Ln().                     //
		Text(string(b.Buffer())). // Data
		Text(footer).             // ---------- [footer]
		Bytes(), nil
}

func (bs *BookmarkStrategy) ParseBuffer(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
	parse := bs.parseBuffer
	if parse == nil {
		parse = defaultParseBuffer
	}
	return parse(ctx, buf, original)
}

func (bs *BookmarkStrategy) Diff(old, fresh *bookmark.Bookmark) string {
	return txt.DiffColorize(txt.Diff(old.Buffer(), fresh.Buffer()))
}

func (bs *BookmarkStrategy) FileType() string { return application.Name }

func bookmarkFromBytes(buf []byte, b *bookmark.Bookmark) {
	lines := strings.Split(string(buf), "\n") // bytes to lines
	b.URL = txt.CleanLines(txt.ExtractBlock(lines, "# *URL:", "# Title:"))
	b.Title = txt.CleanLines(txt.ExtractBlock(lines, "# Title:", "# Tags:"))
	b.Tags = bookmark.ParseTags(txt.CleanLines(txt.ExtractBlock(lines, "# Tags:", "# Description:")))
	b.Desc = txt.CleanLines(txt.ExtractBlock(lines, "# Description:", "# end"))
}

// formatVersion formats the version string.
func formatVersion(v string) string {
	if v == "dev" {
		return v
	}
	return "v" + v
}

func defaultParseBuffer(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
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
