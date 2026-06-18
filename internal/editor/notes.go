package editor

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var _ EditStrategy = (*NotesStrategy)(nil)

type NotesStrategy struct {
	sectionMarker string
}

func (ns *NotesStrategy) BuildBuffer(m *Meta, b *bookmark.Bookmark, idx, total int) ([]byte, error) {
	var (
		bd      = frame.NewBorders("<!-- ", " ", "<!-", "-->")
		f       = frame.New(frame.WithBorders(bd))
		padding = 10
		width   = terminal.MinWidth()
	)

	// content
	separator := txt.SpanCenter(width-len(bd.Header), "", "-")
	idTitle := txt.PaddedLineWithPad("["+strconv.Itoa(b.ID)+"]", txt.Shorten(b.Title, width+padding), 6)
	urlLine := txt.Shorten(b.URL, width-len(bd.Row))
	dbAndVer := fmt.Sprintf("database: %s %s ver: %s", m.DBName, txt.GlyphBulletPoint, formatVersion(m.Version))
	headerFooter := txt.SpanCenter(width-len(bd.Header)-len(bd.Footer), " bookmark notes ", "-")

	ns.sectionMarker = strings.TrimSpace(bd.Header) + txt.NBSP

	return f.
		Headerln(separator).                   // <!-- ------------------
		Rowln(idTitle).                        // [ID] Title
		Rowln(urlLine).                        // URL
		Rowln(dbAndVer).                       // dbName • ver: x.x.x
		Text(ns.sectionMarker + headerFooter). // <!-- --- label ------->
		Footerln().
		Text(b.Notes).
		Bytes(), nil
}

func (ns *NotesStrategy) ParseBuffer(
	ctx context.Context,
	buf []byte,
	og *bookmark.Bookmark,
) (*bookmark.Bookmark, error) {
	editedNotes := txt.ExtractBlockBytes(buf, ns.sectionMarker, "")
	fmt.Printf("%q\n", editedNotes)
	if bytes.Equal([]byte(og.Notes), editedNotes) {
		return nil, ErrBufferUnchanged
	}
	clone := og.Copy()
	clone.Notes = string(editedNotes)
	return clone, nil
}

func (ns *NotesStrategy) Diff(oldB, newB *bookmark.Bookmark) string {
	return txt.DiffColorize(txt.Diff([]byte(oldB.Notes), []byte(newB.Notes)))
}

func (ns *NotesStrategy) Save(ctx context.Context, r *db.SQLite, bm *bookmark.Bookmark) error {
	return r.UpdateOne(ctx, bm)
}

func (ns *NotesStrategy) FileType() string { return "md" }

func NewNotesStrategy() *NotesStrategy {
	return &NotesStrategy{}
}
