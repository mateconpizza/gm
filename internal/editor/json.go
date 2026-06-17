package editor

import (
	"bytes"
	"context"

	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var _ EditStrategy = (*JSONStrategy)(nil)

type JSONStrategy struct{}

func (JSONStrategy) BuildBuffer(m *Meta, b *bookmark.Bookmark, idx, total int) ([]byte, error) {
	return b.Bytes(), nil
}

func (JSONStrategy) ParseBuffer(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
	old := bytes.TrimRight(original.Bytes(), "\n")
	newB := bytes.TrimRight(buf, "\n")

	if bytes.Equal(old, newB) {
		return nil, ErrBufferUnchanged
	}

	bm, err := bookmark.NewFromBuffer(newB)
	if err != nil {
		return nil, err
	}

	return bm, nil
}

func (JSONStrategy) Diff(oldB, newB *bookmark.Bookmark) string {
	return txt.DiffColor(txt.Diff(oldB.Bytes(), newB.Bytes()))
}

func (JSONStrategy) Save(ctx context.Context, r *db.SQLite, bm *bookmark.Bookmark) error {
	return r.UpdateOne(ctx, bm)
}

func (JSONStrategy) FileType() string { return "json" }
