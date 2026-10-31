package editor

import (
	"bytes"
	"context"
	"fmt"

	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var _ EditStrategy = (*JSONStrategy)(nil)

type JSONStrategy struct{}

func NewJSONStrategy() *JSONStrategy { return &JSONStrategy{} }

func (js *JSONStrategy) BuildBuffer(b *bookmark.Bookmark, idx, total int) ([]byte, error) {
	return b.Bytes(), nil
}

func (js *JSONStrategy) ParseBuffer(ctx context.Context, buf []byte, old *bookmark.Bookmark) (*bookmark.Bookmark, error) {
	oldBytes := bytes.TrimRight(old.Bytes(), "\n")
	freshBytes := bytes.TrimRight(buf, "\n")

	if bytes.Equal(oldBytes, freshBytes) {
		return nil, ErrBufferUnchanged
	}

	fresh, err := bookmark.NewFromBuffer(freshBytes)
	if err != nil {
		return nil, err
	}

	if err := bookmark.Validate(fresh); err != nil {
		return nil, err
	}

	if old.ID != fresh.ID {
		return nil, fmt.Errorf("%w: %d", bookmark.ErrBookmarkInvalidID, fresh.ID)
	}

	if old.Equals(fresh) {
		return nil, ErrBufferUnchanged
	}

	return fresh, nil
}

func (js *JSONStrategy) Diff(d Differ, old, fresh *bookmark.Bookmark) string {
	diff := txt.DiffContext(old.Bytes(), fresh.Bytes(), 3)
	return txt.DiffColorize(d, diff)
}

func (js *JSONStrategy) FileType() string { return "json" }
