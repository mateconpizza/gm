package editor

import (
	"bytes"
	"context"
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
	idTitle := "[" + strconv.Itoa(b.ID) + "] " + txt.Shorten(b.Title, width+padding)
	urlLine := txt.Shorten(b.URL, width-len(bd.Row)-padding-6)
	headerFooter := txt.SpanCenter(width-len(bd.Header)-len(bd.Footer), " notes ", "-")

	ns.sectionMarker = strings.TrimSpace(bd.Header) + txt.NBSP

	bullet := func(header, val string) string {
		return txt.PaddedLineWithPad(header+":", val, 11)
	}

	return f.
		Headerln(separator).                              // <!-- ------------------
		Rowln(idTitle).                                   // [ID] Title
		Rowln(bullet("Tags", txt.TagsWithPound(b.Tags))). // Tags:
		Rowln(bullet("URL", urlLine)).                    // URL:
		Rowln(bullet("Database", m.DBName)).              // Database:
		Text(ns.sectionMarker + headerFooter).Footerln(). // <!-- --- label ------->
		Text(b.Notes).                                    // Notes
		Bytes(), nil
}

func (ns *NotesStrategy) ParseBuffer(ctx context.Context, buf []byte, og *bookmark.Bookmark) (*bookmark.Bookmark, error) {
	editedNotes := txt.ExtractBlockBytes(buf, ns.sectionMarker, "")
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
