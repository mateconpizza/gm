package bookmark

import (
	"errors"
	"fmt"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/sys/files"
)

var ErrBufferUnchanged = errors.New("buffer unchanged")

// Edit modifies the provided bookmark based on the given byte slice and text
// editor, returning an error if any operation fails.
func Edit(te *files.TextEditor, bf []byte, b *Bookmark) error {
	if err := editBuffer(te, &bf); err != nil {
		return fmt.Errorf("%w", err)
	}

	var tb *Bookmark
	c := format.ByteSliceToLines(&bf)
	if err := bufferValidate(&c); err != nil {
		return err
	}

	tb = parseContent(&c)
	tb = scrapeAndUpdate(tb)

	tb.ID = b.ID
	*b = *tb

	return nil
}

// editBuffer modifies the byte slice in the text editor and returns an error
// if the edit operation fails.
func editBuffer(te *files.TextEditor, bf *[]byte) error {
	cBf := make([]byte, len(*bf))
	copy(cBf, *bf)

	if err := files.Edit(te, bf); err != nil {
		return fmt.Errorf("%w", err)
	}

	if format.IsSameContentBytes(bf, &cBf) {
		return ErrBufferUnchanged
	}

	return nil
}
