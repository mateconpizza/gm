package bookmark

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/sys/terminal"
)

// Oneline formats a bookmark in a single line with dynamic width adjustment.
func Oneline(b *Bookmark) string {
	const (
		idWithColor    = 16
		minTagsLen     = 34
		defaultTagsLen = 24
		idPadding      = 5
	)
	w := terminal.MaxWidth
	idLen := format.PaddingConditional(idPadding, idWithColor)
	tagsLen := format.PaddingConditional(minTagsLen, defaultTagsLen)
	// calculate url length dynamically based on available space
	// add 1 for the UnicodeMiddleDot spacer
	urlLen := w - idLen - tagsLen - 1
	// apply colors
	coloredID := color.BrightYellow(b.ID).Bold().String()
	shortURL := format.Shorten(b.URL, urlLen)
	colorURL := color.Gray(shortURL).String()
	// adjust for ansi color codes in url length calculation
	urlLen += len(colorURL) - len(shortURL)
	// process and color tags
	tagsColor := color.BrightCyan(prettifyTags(b.Tags)).Italic().String()
	var sb strings.Builder
	sb.Grow(w + 20) // pre-allocate buffer with some extra space for color codes
	sb.WriteString(fmt.Sprintf("%-*s ", idLen, coloredID))
	sb.WriteString(format.UnicodeMiddleDot)
	sb.WriteString(fmt.Sprintf(" %-*s %-*s\n", urlLen, colorURL, tagsLen, tagsColor))

	return sb.String()
}

// Multiline formats a bookmark for fzf with max width.
func Multiline(b *Bookmark) string {
	w := terminal.MaxWidth
	var sb strings.Builder
	sb.WriteString(color.BrightYellow(b.ID).Bold().String())
	sb.WriteString(format.NBSP)
	sb.WriteString(format.Shorten(breadcrumbsURL(b.URL, color.BrightMagenta), w) + "\n")
	sb.WriteString(color.Cyan(format.Shorten(b.Title, w)).String() + "\n")
	sb.WriteString(color.BrightGray(prettifyTags(b.Tags)).Italic().String())

	return sb.String()
}

func FrameFormatted(b *Bookmark, c color.ColorFn) string {
	f := frame.New(frame.WithColorBorder(c))
	w := terminal.MaxWidth
	w -= len(f.Border.Row)
	// split
	descSplit := format.SplitIntoChunks(b.Desc, w)
	titleSplit := format.SplitIntoChunks(b.Title, w)
	// add color and style
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := format.Shorten(breadcrumbsURL(b.URL, color.BrightMagenta), w)
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.Gray)
	tags := color.Gray(prettifyTags(b.Tags)).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln().
		Mid(title...).Ln().
		Mid(desc...).Ln().
		Footer(tags).Ln().
		String()
}

// Frame formats a bookmark in a frame with min width.
func Frame(b *Bookmark) string {
	w := terminal.MaxWidth
	f := frame.New(frame.WithColorBorder(color.Gray))
	// indentation
	w -= len(f.Border.Row)
	// split and add intendation
	descSplit := format.SplitIntoChunks(b.Desc, w)
	titleSplit := format.SplitIntoChunks(b.Title, w)
	// add color and style
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := format.Shorten(breadcrumbsURL(b.URL, color.BrightMagenta), w)
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.Gray)
	tags := color.BrightGray(prettifyTags(b.Tags)).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln().
		Mid(title...).Ln().
		Mid(desc...).Ln().
		Footer(tags).Ln().
		String()
}

// prettifyTags returns a prettified tags.
func prettifyTags(s string) string {
	uc := format.UnicodeMiddleDot
	return strings.TrimRight(strings.ReplaceAll(s, ",", uc), uc)
}

// breadcrumbsURL returns a prettified URL with color.
func breadcrumbsURL(s string, c color.ColorFn) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	// default color
	if c == nil {
		c = color.Default
	}
	if u.Host == "" || u.Path == "" {
		return c(s).Bold().String()
	}
	host := c(u.Host).Bold().String()
	pathSegments := strings.FieldsFunc(
		strings.TrimLeft(u.Path, "/"),
		func(r rune) bool { return r == '/' },
	)

	if len(pathSegments) == 0 {
		return host
	}

	uc := format.UnicodeSingleAngleMark
	segments := strings.Join(pathSegments, fmt.Sprintf(" %s ", uc))
	pathSeg := color.Text(uc, segments).Italic()

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// FzfFormatter returns a function to format a bookmark for the FZF menu.
func FzfFormatter(m bool) func(b *Bookmark) string {
	if m {
		return Multiline
	}

	return Oneline
}
