package editor

import (
	"bytes"
	"context"

	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var _ EditStrategy = (*JSONStrategy)(nil)

type JSONStrategy struct{}

func NewJSONStrategy() *JSONStrategy {
	return &JSONStrategy{}
}

func (JSONStrategy) BuildBuffer(m *Meta, b *bookmark.Bookmark, idx, total int) ([]byte, error) {
	return b.Bytes(), nil
}

func (JSONStrategy) ParseBuffer(ctx context.Context, buf []byte, original *bookmark.Bookmark) (*bookmark.Bookmark, error) {
	old := bytes.TrimRight(original.Bytes(), "\n")
	fresh := bytes.TrimRight(buf, "\n")

	if bytes.Equal(old, fresh) {
		return nil, ErrBufferUnchanged
	}

	bm, err := bookmark.NewFromBuffer(fresh)
	if err != nil {
		return nil, err
	}

	return bm, nil
}

func (JSONStrategy) Diff(oldB, newB *bookmark.Bookmark) string {
	return txt.DiffColorize(txt.Diff(oldB.Bytes(), newB.Bytes()))
}

func (JSONStrategy) FileType() string { return "json" }
