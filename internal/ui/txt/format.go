package txt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// Oneline formats a bookmark in a single line with the given colorscheme.
func Oneline(c *ui.Console, b *bookmark.Bookmark) string {
	w := terminal.MaxWidth

	const (
		idPadding      = 3
		idWithColor    = 4 // visible width for IDS up to 9999
		defaultTagsLen = 24
		minTagsLen     = 34
	)

	idLen := idPadding
	tagsLen := minTagsLen

	p := c.Palette()
	if !p.Enabled() {
		idLen = idWithColor
		tagsLen = defaultTagsLen
	}

	// ID padding con color sin romper el formato
	idStr := strconv.Itoa(b.ID)
	paddedID := fmt.Sprintf("%*s", idLen, idStr)
	coloredID := strings.Replace(paddedID, idStr, p.BrightYellow.Wrap(idStr, p.Bold), 1)

	// Calculate long available for URL
	const urlPadding = 3 // 3 = ' ' + '·' + ' '.
	urlLen := w - idLen - urlPadding - tagsLen
	shortURL := Shorten(b.URL, urlLen)
	colorURL := p.BrightWhite.Sprint(shortURL)
	urlLen += len(colorURL) - len(shortURL)

	// tags
	tagsColor := p.Blue.Wrap(TagsWithUnicode(b.Tags), p.Italic)

	sep := " " + UnicodeMiddleDot + " "
	if b.Notes != "" {
		sep = p.BrightMagenta.Wrap(" "+UnicodeBulletPoint+" ", p.Bold)
	}

	var sb strings.Builder
	sb.Grow(w + 20)
	sb.WriteString(coloredID)
	sb.WriteString(sep)
	sb.WriteString(fmt.Sprintf("%-*s %-*s\n", urlLen, colorURL, tagsLen, tagsColor))

	return sb.String()
}

// Multiline formats a bookmark for fzf with max width.
func Multiline(c *ui.Console, b *bookmark.Bookmark) string {
	p := c.Palette()
	w := terminal.MaxWidth

	var sb strings.Builder
	sb.WriteString(p.BrightYellow.With(p.Bold).Sprint(b.ID))
	sb.WriteString(NBSP)
	sb.WriteString(Shorten(URLBreadCrumbsColor(p, b.URL, UnicodeSingleAngleMark), w) + "\n")

	if b.Title != "" {
		sb.WriteString(p.Cyan.Sprint(Shorten(b.Title, w)) + "\n")
	}

	sb.WriteString(p.BrightWhite.Wrap(TagsWithUnicode(b.Tags), p.Italic))

	return sb.String()
}

func FrameFormatted(c *ui.Console, b *bookmark.Bookmark) string {
	p := c.Palette()
	f := frame.New(frame.WithColorBorder(ansi.BrightBlack))
	w := terminal.MaxWidth - len(f.Border.Row)

	// id + url
	id := p.BrightYellow.With(p.Bold).Sprint(b.ID)
	urlColor := Shorten(URLBreadCrumbsColor(p, b.URL, UnicodeSingleAngleMark), w)
	f.Headerln(fmt.Sprintf("%s %s", id, urlColor))

	// title
	if b.Title != "" {
		f.Midln(ansi.StyleAll(SplitIntoChunks(b.Title, w), p.Cyan)...)
	}

	// description
	if b.Desc != "" {
		f.Midln(ansi.StyleAll(SplitIntoChunks(b.Desc, w), p.BrightBlack)...)
	}

	// tags
	tags := p.BrightBlack.With(p.Italic).Sprint(TagsWithPound(b.Tags))
	f.Footer(tags).Ln()

	return f.String()
}

// Frame formats a bookmark in a frame with min width.
func Frame(c *ui.Console, b *bookmark.Bookmark) string {
	w := terminal.MinWidth
	p := c.Palette()
	f := c.Frame()

	// indentation
	w -= len(f.Border.Row)

	// id + url
	id := p.BrightYellow.With(p.Bold).Sprint(b.ID)
	urlColor := Shorten(URLBreadCrumbsColor(p, b.URL, UnicodeSingleAngleMark), w)
	f.Headerln(fmt.Sprintf("%s %s", id, urlColor))

	// title
	if b.Title != "" {
		titleSplit := SplitIntoChunks(b.Title, w)
		f.Midln(ansi.StyleAll(titleSplit, p.BrightCyan)...)
	}

	// description
	if b.Desc != "" {
		descSplit := SplitIntoChunks(b.Desc, w)
		f.Midln(ansi.StyleAll(descSplit, p.BrightBlack)...)
	}

	// tags
	f.Mid(TagsWithColorPound(c, b.Tags)).Ln()

	return f.StringReset()
}

func Notes(c *ui.Console, b *bookmark.Bookmark) string {
	w := terminal.MinWidth
	p := c.Palette()
	f := frame.New(frame.WithColorBorder(ansi.BrightBlack))

	// id + url
	id := p.BrightYellow.With(p.Bold).Sprint(b.ID)
	urlColor := Shorten(URLBreadCrumbsColor(p, b.URL, UnicodeSingleAngleMark), w)
	f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln()

	// notes
	notes := SplitIntoChunks(b.Notes, w)
	f.Footerln(ansi.StyleAll(notes, p.White)...)

	return f.String()
}
