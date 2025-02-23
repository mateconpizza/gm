package bookmark

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/sys/files"
)

var ErrBufferUnchanged = errors.New("buffer unchanged")

// Edit modifies the provided bookmark based on the given byte slice and text
// editor, returning an error if any operation fails.
func Edit(te *files.TextEditor, content []byte, b *Bookmark) error {
	original := bytes.Clone(content)
	data, err := te.EditContentBytes(content)
	if err != nil {
		return fmt.Errorf("failed to edit content: %w", err)
	}
	if bytes.Equal(data, original) {
		return ErrBufferUnchanged
	}
	lines := format.ByteSliceToLines(data)
	if err := validateBookmarkFormat(lines); err != nil {
		return fmt.Errorf("invalid bookmark format: %w", err)
	}
	tb := parseBookmarkContent(lines)
	tb = scrapeBookmark(tb)
	tb.ID = b.ID
	*b = *tb

	return nil
}
