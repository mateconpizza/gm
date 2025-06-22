package bookmark

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

var ErrBufferUnchanged = errors.New("buffer unchanged")

// BookmarkEdit holds information about a bookmark edit operation.
type BookmarkEdit struct {
	item   *Bookmark
	header []byte
	body   []byte
	footer []byte
	idx    int
	total  int
}

func newBookmarkEdit(b *Bookmark) *BookmarkEdit {
	return &BookmarkEdit{
		item: b,
		body: b.Buffer(),
	}
}

func (be *BookmarkEdit) Buffer() []byte {
	buf := make([]byte, 0, len(be.header)+len(be.body)+len(be.footer))
	buf = append(buf, be.header...)
	buf = append(buf, be.body...)
	buf = append(buf, be.footer...)

	return buf
}

// Edit edits a bookmark and validates the resulting content.
func Edit(te *files.TextEditor, b *Bookmark, idx, total int) (*Bookmark, error) {
	be := newBookmarkEdit(b)
	be.idx = idx
	be.total = total

	original := bytes.Clone(be.body)

	prepareBufferForEdition(be)

	modifiedData, err := te.EditBytes(be.Buffer(), config.App.Name)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if bytes.Equal(modifiedData, original) {
		return nil, ErrBufferUnchanged
	}

	lines := strings.Split(string(modifiedData), "\n") // bytes to lines
	if err := validateBookmarkFormat(lines); err != nil {
		return nil, fmt.Errorf("invalid bookmark format: %w", err)
	}

	tb := parseBookmarkContent(lines)
	if be.item.Equals(tb) {
		return nil, ErrBufferUnchanged
	}

	tb = scrapeBookmark(tb)
	tb.ID = be.item.ID
	tb.CreatedAt = be.item.CreatedAt
	tb.Favorite = be.item.Favorite
	tb.LastVisit = be.item.LastVisit
	tb.VisitCount = be.item.VisitCount

	return tb, nil
}

// prepareBufferForEdition prepares the buffer for edition.
func prepareBufferForEdition(be *BookmarkEdit) {
	const spaces = 10

	newBookmark := be.item.ID == 0

	// header
	shortTitle := txt.Shorten(be.item.Title, terminal.MinWidth-spaces-6)

	header := fmt.Appendf(nil, "# %d %s\n#\n", be.item.ID, shortTitle)
	if newBookmark {
		header = fmt.Appendf(nil, "# %s\n#\n", shortTitle)
	}

	// header mesg
	s := "bookmark edition"
	if newBookmark {
		s = "bookmark addition"
	}

	sep := txt.CenteredLine(terminal.MinWidth-spaces, s)

	// metadata
	meta := fmt.Appendf(nil,
		"# database:\t%q\n# version:\tv%s\n# %s\n\n",
		config.App.DBName,
		config.App.Info.Version,
		sep,
	)

	// footer
	be.footer = fmt.Appendf(nil, " [%d/%d]", be.idx+1, be.total)
	if newBookmark {
		be.footer = fmt.Appendf(nil, " [New]")
	}

	// assemble
	header = append(header, meta...)
	be.header = append(be.header, header...)
}
