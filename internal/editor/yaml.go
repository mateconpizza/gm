package editor

import (
	"bytes"
	"context"
	"fmt"

	yaml "gopkg.in/yaml.v3"

	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var _ EditStrategy = (*YAMLStrategy)(nil)

type YAMLStrategy struct {
	marshal   func(in any) (out []byte, err error)
	unmarshal func(in []byte, out any) (err error)
}

func NewYAMLStrategy() *YAMLStrategy {
	return &YAMLStrategy{
		marshal:   yaml.Marshal,
		unmarshal: yaml.Unmarshal,
	}
}

func (ys *YAMLStrategy) BuildBuffer(b *bookmark.Bookmark, idx, total int) ([]byte, error) {
	return ys.marshal(b.Record())
}

func (ys *YAMLStrategy) ParseBuffer(ctx context.Context, buf []byte, old *bookmark.Bookmark) (*bookmark.Bookmark, error) {
	oldBytes, err := ys.marshal(old.Record())
	if err != nil {
		return nil, err
	}

	if bytes.Equal(oldBytes, buf) {
		return nil, ErrBufferUnchanged
	}

	by := bookmark.NewRecord()
	if err := ys.unmarshal(buf, by); err != nil {
		return nil, err
	}

	fresh := by.Bookmark()
	if err := bookmark.Validate(fresh); err != nil {
		return nil, err
	}

	if old.ID != fresh.ID {
		return nil, fmt.Errorf("%w: %d", bookmark.ErrBookmarkInvalidID, fresh.ID)
	}

	if fresh.Equals(old) {
		return nil, ErrBufferUnchanged
	}

	return fresh, nil
}

func (ys *YAMLStrategy) Diff(d Differ, old, fresh *bookmark.Bookmark) string {
	oldBuf, _ := ys.marshal(old.Record())
	freshBuf, _ := ys.marshal(fresh.Record())
	diff := txt.DiffContext(oldBuf, freshBuf, 3)
	return txt.DiffColorize(d, diff)
}

func (ys *YAMLStrategy) FileType() string { return "yaml" }
