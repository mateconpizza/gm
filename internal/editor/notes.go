package editor

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
)

type NotesStrategy struct{}

func (NotesStrategy) BuildBuffer(b *Record, idx, total int) ([]byte, error) {
	w := width - rightMargin

	buf := NewBufferBuilder(b)
	buf.Body = b.BufferNotes()
	buf.Idx, buf.Total = idx, total

	titleSplit := txt.SplitIntoChunks(b.Title, w-idFieldWidth)
	shortTitle := strings.Join(titleSplit, "\n# ")
	shortTitle += "\n#\n# " + txt.Shorten(b.URL, w)
	header := fmt.Appendf(nil, "# %d %s\n#\n", b.ID, shortTitle)

	// metadata
	cfg := config.New()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	sep := txt.CenteredLine(w, "bookmark notes", "-")
	meta := fmt.Appendf(nil, "# database:\t%q\n# version:\tv%s\n# %s\n\n", cfg.DBName, cfg.Info.Version, sep)

	buf.Header = append(buf.Header, header...)
	buf.Header = append(buf.Header, meta...)

	// Similar to your EditNotes logic
	return buf.Buffer(), nil
}

func (NotesStrategy) ParseBuffer(ctx context.Context, buf []byte, original *Record, idx, total int) (*Record, error) {
	editedNotes := txt.ExtractBlockBytes(buf, "# Notes", "")
	if bytes.Equal([]byte(original.Notes), editedNotes) {
		return nil, ErrBufferUnchanged
	}
	clone := *original
	clone.Notes = string(editedNotes)
	return &clone, nil
}

func (NotesStrategy) Diff(oldB, newB *Record) string {
	return txt.DiffColor(txt.Diff([]byte(oldB.Notes), []byte(newB.Notes)))
}

func (NotesStrategy) Save(ctx context.Context, r *db.SQLite, bm *Record) error {
	return r.UpdateOne(ctx, bm)
}

func (NotesStrategy) EditType() string {
	return config.New().Name
}
