package editor

import (
	"context"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var width = terminal.MinWidth

const (
	rightMargin  = 4 // padding on the right side of the buffer
	idFieldWidth = 4 // space reserved for the numeric bookmark ID
)

type baseBookmarkStrategy struct{}

func (baseBookmarkStrategy) BuildBuffer(b *Record, idx, total int) ([]byte, error) {
	buf := NewBufferBuilder(b)
	buf.Idx, buf.Total = idx, total

	titleSplit := txt.SplitIntoChunks(buf.Item.Title, width-rightMargin)
	shortTitle := strings.Join(titleSplit, "\n# ")

	isNewBookmark := buf.Item.ID == 0
	header := fmt.Appendf(nil, "# %d %s\n#\n", buf.Item.ID, shortTitle)
	if isNewBookmark {
		header = fmt.Appendf(nil, "# %s\n#\n", shortTitle)
	}

	// header mesg
	s := "bookmark edition"
	if isNewBookmark {
		s = "bookmark addition"
	}

	sep := txt.CenteredLine(width-rightMargin, s, "-")

	// metadata
	cfg := config.New()
	meta := fmt.Appendf(nil, "# database:\t%q\n# version:\tv%s\n# %s\n\n", cfg.DBName, cfg.Info.Version, sep)

	// footer
	buf.Footer = fmt.Appendf(nil, " [%d/%d]", buf.Idx+1, buf.Total)
	if isNewBookmark {
		buf.Footer = fmt.Appendf(nil, " [New]")
	}

	// assemble
	header = append(header, meta...)
	buf.Header = append(buf.Header, header...)

	return buf.Buffer(), nil
}

func (baseBookmarkStrategy) ParseBuffer(buf []byte, original *Record, idx, total int) (*Record, error) {
	edited := bookmarkFromBytes(buf)
	if err := bookmark.Validate(edited); err != nil {
		return nil, err
	}

	if original.Equals(edited) {
		return nil, ErrBufferUnchanged
	}

	edited = metadata.EnrichBookmark(edited)
	edited.ID = original.ID
	edited.CreatedAt = original.CreatedAt
	edited.Favorite = original.Favorite
	edited.LastVisit = original.LastVisit
	edited.VisitCount = original.VisitCount
	edited.FaviconURL = original.FaviconURL

	return edited, nil
}

func (baseBookmarkStrategy) Diff(oldB, newB *Record) string {
	return txt.DiffColor(txt.Diff(oldB.Buffer(), newB.Buffer()))
}

func (baseBookmarkStrategy) EditType() string {
	return config.New().Name
}

// BookmarkStrategy implements the Strategy interface for editing
// existing bookmarks.
type BookmarkStrategy struct {
	baseBookmarkStrategy
}

func (BookmarkStrategy) Save(ctx context.Context, r *db.SQLite, bm *Record) error {
	return r.UpdateOne(ctx, bm)
}

// NewBookmarkStrategy implements the Strategy interface for
// creating new bookmarks.
type NewBookmarkStrategy struct {
	baseBookmarkStrategy
}

func (NewBookmarkStrategy) Save(ctx context.Context, r *db.SQLite, bm *Record) error {
	_, err := r.InsertOne(ctx, bm)
	if err != nil {
		return err
	}

	return nil
}
