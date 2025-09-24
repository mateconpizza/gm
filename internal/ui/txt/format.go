package txt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// Oneline formats a bookmark in a single line with the given colorscheme.
func Oneline(b *bookmark.Bookmark) string {
	w := terminal.MaxWidth

	const (
		idPadding      = 3
		idWithColor    = 4 // visible width for IDS up to 9999
		defaultTagsLen = 24
		minTagsLen     = 34
	)

	idLen := idPadding
	tagsLen := minTagsLen

	cs := color.DefaultColorScheme()
	if cs.Enabled {
		idLen = idWithColor
		tagsLen = defaultTagsLen
	}

	// ID padding con color sin romper el formato
	idStr := strconv.Itoa(b.ID)
	paddedID := fmt.Sprintf("%*s", idLen, idStr)
	coloredID := strings.Replace(paddedID, idStr, cs.BrightYellow(idStr).Bold().String(), 1)

	// Calculate long available for URL
	const urlPadding = 3 // 3 = ' ' + '·' + ' '.
	urlLen := w - idLen - urlPadding - tagsLen
	shortURL := Shorten(b.URL, urlLen)
	colorURL := cs.BrightWhite(shortURL).String()
	urlLen += len(colorURL) - len(shortURL)

	// tags
	tagsColor := cs.Blue(TagsWithUnicode(b.Tags)).Italic().String()

	sep := " " + UnicodeMiddleDot + " "
	if b.Notes != "" {
		sep = cs.BrightMagenta(" " + UnicodeBulletPoint + " ").Bold().String()
	}

	var sb strings.Builder
	sb.Grow(w + 20)
	sb.WriteString(coloredID)
	sb.WriteString(sep)
	sb.WriteString(fmt.Sprintf("%-*s %-*s\n", urlLen, colorURL, tagsLen, tagsColor))

	return sb.String()
}

// Multiline formats a bookmark for fzf with max width.
func Multiline(b *bookmark.Bookmark) string {
	w := terminal.MaxWidth

	var sb strings.Builder

	cs := color.DefaultColorScheme()
	sb.WriteString(cs.BrightYellow(b.ID).Bold().String())
	sb.WriteString(NBSP)
	sb.WriteString(Shorten(URLBreadCrumbsColor(b.URL, cs.BrightMagenta), w) + "\n")

	if b.Title != "" {
		sb.WriteString(cs.Cyan(Shorten(b.Title, w)).String() + "\n")
	}

	sb.WriteString(cs.BrightWhite(TagsWithUnicode(b.Tags)).Italic().String())

	return sb.String()
}

func FrameFormatted(b *bookmark.Bookmark, c color.ColorFn) string {
	f := frame.New(frame.WithColorBorder(c))
	w := terminal.MaxWidth - len(f.Border.Row)
	// id + url
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := Shorten(URLBreadCrumbsColor(b.URL, color.BrightMagenta), w)
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
	tags := color.Gray(TagsWithPound(b.Tags)).Italic().String()
	f.Footer(tags).Ln()

	return f.String()
}

// Frame formats a bookmark in a frame with min width.
func Frame(b *bookmark.Bookmark) string {
	w := terminal.MinWidth
	cs := color.DefaultColorScheme()
	f := frame.New(frame.WithColorBorder(cs.BrightBlack))

	// indentation
	w -= len(f.Border.Row)

	// id + url
	id := cs.BrightYellow(b.ID).Bold()
	urlColor := Shorten(URLBreadCrumbsColor(b.URL, cs.BrightMagenta), w) + color.Reset()
	f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln()

	// title
	if b.Title != "" {
		titleSplit := SplitIntoChunks(b.Title, w)
		title := color.ApplyMany(titleSplit, cs.BrightCyan)
		f.Midln(title...)
	}

	// description
	if b.Desc != "" {
		descSplit := SplitIntoChunks(b.Desc, w)
		desc := color.ApplyMany(descSplit, cs.White)
		f.Mid(desc...).Ln()
	}

	// tags
	tags := cs.BrightWhite(TagsWithPound(b.Tags)).Italic().String()
	f.Mid(tags).Ln()

	// notes
	if b.Notes != "" {
		notes := color.ApplyMany(SplitIntoChunks(b.Notes, w), color.StyleDim)
		f.Footerln(notes...)
	}

	return f.String()
}

func Notes(b *bookmark.Bookmark) string {
	w := terminal.MinWidth
	cs := color.DefaultColorScheme()
	f := frame.New(frame.WithColorBorder(cs.BrightBlack))

	// id + url
	id := cs.BrightYellow(b.ID).Bold()
	urlColor := Shorten(URLBreadCrumbsColor(b.URL, cs.BrightMagenta), w) + color.Reset()
	f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln()

	// notes
	notes := SplitIntoChunks(b.Notes, w)
	f.Footerln(color.ApplyMany(notes, cs.White)...)

	return f.String()
}
