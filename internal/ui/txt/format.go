package txt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
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
	if !p.Disable() {
		idLen = idWithColor
		tagsLen = defaultTagsLen
	}

	// ID padding con color sin romper el formato
	idStr := strconv.Itoa(b.ID)
	paddedID := fmt.Sprintf("%*s", idLen, idStr)
	coloredID := strings.Replace(paddedID, idStr, p.BrightYellowBold(idStr), 1)

	// Calculate long available for URL
	const urlPadding = 3 // 3 = ' ' + '·' + ' '.
	urlLen := w - idLen - urlPadding - tagsLen
	shortURL := Shorten(b.URL, urlLen)
	colorURL := p.BrightWhite(shortURL)
	urlLen += len(colorURL) - len(shortURL)

	// tags
	tagsColor := p.BlueItalic(TagsWithUnicode(b.Tags))

	sep := " " + UnicodeMiddleDot + " "
	if b.Notes != "" {
		sep = p.BrightMagentaBold(" " + UnicodeBulletPoint + " ")
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
	sb.WriteString(p.BrightYellowBold(b.ID))
	sb.WriteString(NBSP)
	sb.WriteString(Shorten(URLBreadCrumbsColor(p, b.URL, UnicodeSingleAngleMark), w) + "\n")

	if b.Title != "" {
		sb.WriteString(p.Cyan(Shorten(b.Title, w)) + "\n")
	}

	sb.WriteString(p.BrightWhiteItalic(TagsWithUnicode(b.Tags)))

	return sb.String()
}

func FrameFormatted(c *ui.Console, b *bookmark.Bookmark) string {
	p := c.Palette()
	f := frame.New(frame.WithColorBorder(frame.ColorGray))
	w := terminal.MaxWidth - len(f.Border.Row)

	// id + url
	id := p.BrightYellowBold(b.ID)
	urlColor := Shorten(URLBreadCrumbsColor(p, b.URL, UnicodeSingleAngleMark), w)
	f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln()

	// title
	if b.Title != "" {
		titleSplit := SplitIntoChunks(b.Title, w)
		title := color.ApplyMany(titleSplit, color.Cyan)
		f.Mid(title...).Ln()
	}

	// description
	if b.Desc != "" {
		descSplit := SplitIntoChunks(b.Desc, w)
		desc := color.ApplyMany(descSplit, color.Gray)
		f.Mid(desc...).Ln()
	}

	// tags
	tags := p.GrayItalic(TagsWithPound(b.Tags))
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
	id := p.BrightYellowBold(b.ID)
	urlColor := Shorten(URLBreadCrumbsColor(p, b.URL, UnicodeSingleAngleMark), w) + color.Reset()
	f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln()

	// title
	if b.Title != "" {
		titleSplit := SplitIntoChunks(b.Title, w)
		title := color.ApplyMany(titleSplit, color.BrightCyan)
		f.Midln(title...)
	}

	// description
	if b.Desc != "" {
		descSplit := SplitIntoChunks(b.Desc, w)
		desc := color.ApplyMany(descSplit, color.White)
		f.Mid(desc...).Ln()
	}

	// tags
	f.Mid(TagsWithColorPound(c, b.Tags)).Ln()

	return f.StringReset()
}

func Notes(c *ui.Console, b *bookmark.Bookmark) string {
	w := terminal.MinWidth
	p := c.Palette()
	f := frame.New(frame.WithColorBorder(frame.ColorBrightBlack))

	// id + url
	id := p.BrightYellowBold(b.ID)
	urlColor := Shorten(URLBreadCrumbsColor(p, b.URL, UnicodeSingleAngleMark), w) + color.Reset()
	f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln()

	// notes
	notes := SplitIntoChunks(b.Notes, w)
	f.Footerln(color.ApplyMany(notes, color.White)...)

	return f.String()
}
