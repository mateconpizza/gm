package bookmark

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

var ErrBufferUnchanged = errors.New("buffer unchanged")

// Edit modifies the provided bookmark based on the given byte slice and text
// editor, returning an error if any operation fails.
func Edit(te *files.TextEditor, t *terminal.Term, content []byte, b *Bookmark) error {
	original := bytes.Clone(content)
	var tb *Bookmark
	for {
		modifiedData, err := te.EditBytes(content, config.App.Name)
		if err != nil {
			return fmt.Errorf("failed to edit content: %w", err)
		}
		if bytes.Equal(modifiedData, original) {
			return ErrBufferUnchanged
		}
		lines := format.ByteSliceToLines(modifiedData)
		if err := validateBookmarkFormat(lines); err != nil {
			return fmt.Errorf("invalid bookmark format: %w", err)
		}

		tb = parseBookmarkContent(lines)
		if b.Equals(tb) {
			return ErrBufferUnchanged
		}
		tb = scrapeBookmark(tb)
		tb.ID = b.ID
		tb.CreatedAt = b.CreatedAt
		tb.Favorite = b.Favorite
		tb.LastVisit = b.LastVisit
		tb.VisitCount = b.VisitCount

		f := frame.New(frame.WithColorBorder(color.BrightBlue))
		f.Header(color.BrightYellow("Edit Bookmark:\n\n").String()).Flush()
		diff := te.Diff(b.Buffer(), tb.Buffer())
		fmt.Println(format.DiffColor(diff))

		opt, err := t.Choose(
			f.Clear().Question("save changes?").String(),
			[]string{"yes", "no", "edit"},
			"y",
		)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		switch strings.ToLower(opt) {
		case "y", "yes":
			*b = *tb
			return nil
		case "n", "no":
			return sys.ErrActionAborted
		case "e", "edit":
			content = modifiedData
			fmt.Println()
			continue
		}
	}
}
