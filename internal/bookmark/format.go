package bookmark

import (
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

// Oneline formats a bookmark in a single line with the given colorscheme.
func Oneline(b *Bookmark, cs *color.Scheme) string {
	w := terminal.MaxWidth
	const (
		idPadding      = 3
		idWithColor    = 16
		defaultTagsLen = 24
		minTagsLen     = 34
	)
	idLen := idPadding
	tagsLen := minTagsLen
	if cs.Enabled {
		idLen = idWithColor
		tagsLen = defaultTagsLen
	}
	// calculate url length dynamically based on available space add 1 for the
	// UnicodeMiddleDot spacer
	urlLen := w - idLen - tagsLen - 1
	// apply colors
	coloredID := cs.BrightYellow(b.ID).Bold().String()
	shortURL := format.Shorten(b.URL, urlLen)
	colorURL := cs.BrightWhite(shortURL).String()
	// adjust for ansi color codes in url length calculation
	urlLen += len(colorURL) - len(shortURL)
	// process and color tags
	tagsColor := cs.Blue(format.TagsWithUnicode(b.Tags)).Italic().String()
	var sb strings.Builder
	sb.Grow(w + 20) // pre-allocate buffer with some extra space for color codes
	sb.WriteString(fmt.Sprintf("%-*s ", idLen, coloredID))
	sb.WriteString(format.UnicodeMiddleDot)
	sb.WriteString(fmt.Sprintf(" %-*s %-*s\n", urlLen, colorURL, tagsLen, tagsColor))

	return sb.String()
}

// Multiline formats a bookmark for fzf with max width.
func Multiline(b *Bookmark, cs *color.Scheme) string {
	w := terminal.MaxWidth
	var sb strings.Builder
	sb.WriteString(cs.BrightYellow(b.ID).Bold().String())
	sb.WriteString(format.NBSP)
	sb.WriteString(format.Shorten(format.URLBreadCrumbs(b.URL, cs.BrightMagenta), w) + "\n")
	if b.Title != "" {
		sb.WriteString(cs.Cyan(format.Shorten(b.Title, w)).String() + "\n")
	}
	sb.WriteString(cs.BrightWhite(format.TagsWithUnicode(b.Tags)).Italic().String())

	return sb.String()
}

func FrameFormatted(b *Bookmark, c color.ColorFn) string {
	f := frame.New(frame.WithColorBorder(c))
	w := terminal.MaxWidth
	w -= len(f.Border.Row)
	// id + url
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := format.Shorten(format.URLBreadCrumbs(b.URL, color.BrightMagenta), w)
	f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln()
	// title
	if b.Title != "" {
		titleSplit := format.SplitIntoChunks(b.Title, w)
		title := color.ApplyMany(titleSplit, color.Cyan)
		f.Mid(title...).Ln()
	}
	// description
	if b.Desc != "" {
		descSplit := format.SplitIntoChunks(b.Desc, w)
		desc := color.ApplyMany(descSplit, color.Gray)
		f.Mid(desc...).Ln()
	}
	// tags
	tags := color.Gray(format.TagsWithPound(b.Tags)).Italic().String()
	f.Footer(tags).Ln()

	return f.String()
}

// Frame formats a bookmark in a frame with min width.
func Frame(b *Bookmark, cs *color.Scheme) string {
	w := terminal.MinWidth
	f := frame.New(frame.WithColorBorder(cs.BrightBlack))
	// indentation
	w -= len(f.Border.Row)
	// id + url
	id := cs.BrightYellow(b.ID).Bold()
	urlColor := format.Shorten(format.URLBreadCrumbs(b.URL, cs.BrightMagenta), w) + color.Reset()
	f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln()
	// title
	if b.Title != "" {
		titleSplit := format.SplitIntoChunks(b.Title, w)
		title := color.ApplyMany(titleSplit, cs.BrightCyan)
		f.Mid(title...).Ln()
	}
	// description
	if b.Desc != "" {
		descSplit := format.SplitIntoChunks(b.Desc, w)
		desc := color.ApplyMany(descSplit, cs.White)
		f.Mid(desc...).Ln()
	}
	// tags
	tags := cs.BrightWhite(format.TagsWithPound(b.Tags)).Italic().String()
	f.Footer(tags).Ln()

	return f.String()
}
