package editor

import (
	"errors"
	"strings"

	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrLineNotFound    = errors.New("line not found")
	ErrTagsEmpty       = errors.New("tags cannot be empty")
	ErrURLEmpty        = errors.New("URL cannot be empty")
	ErrBufferUnchanged = errors.New("buffer unchanged")
)

// BufferBuilder holds information about a bookmark edit operation.
type BufferBuilder struct {
	Item   *bookmark.Bookmark
	Header []byte
	Body   []byte
	Footer []byte
	Idx    int
	Total  int
}

func NewBufferBuilder(b *bookmark.Bookmark) *BufferBuilder {
	return &BufferBuilder{
		Item: b,
		Body: b.Buffer(),
	}
}

func (be *BufferBuilder) Buffer() []byte {
	buf := make([]byte, 0, len(be.Header)+len(be.Body)+len(be.Footer))
	buf = append(buf, be.Header...)
	buf = append(buf, be.Body...)
	buf = append(buf, be.Footer...)

	return buf
}

func bookmarkFromBytes(buf []byte) *bookmark.Bookmark {
	lines := strings.Split(string(buf), "\n") // bytes to lines
	b := bookmark.New()
	b.URL = txt.CleanLines(txt.ExtractBlock(lines, "# URL:", "# Title:"))
	b.Title = txt.CleanLines(txt.ExtractBlock(lines, "# Title:", "# Tags:"))
	b.Tags = bookmark.ParseTags(txt.CleanLines(txt.ExtractBlock(lines, "# Tags:", "# Description:")))
	b.Desc = txt.CleanLines(txt.ExtractBlock(lines, "# Description:", "# end"))

	return b
}
