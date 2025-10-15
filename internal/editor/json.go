package editor

import (
	"bytes"
	"context"

	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type JSONStrategy struct{}

func (JSONStrategy) BuildBuffer(b *Record, idx, total int) ([]byte, error) {
	return b.Bytes(), nil
}

func (JSONStrategy) ParseBuffer(ctx context.Context, buf []byte, original *Record, idx, total int) (*Record, error) {
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

func (JSONStrategy) Diff(oldB, newB *Record) string {
	return txt.DiffColor(txt.Diff(oldB.Bytes(), newB.Bytes()))
}

func (JSONStrategy) Save(ctx context.Context, r *db.SQLite, bm *Record) error {
	return r.UpdateOne(ctx, bm)
}

func (JSONStrategy) EditType() string {
	return "json"
}
