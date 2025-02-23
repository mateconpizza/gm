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

// Oneline formats a bookmark in a single line with max width.
func Oneline(b *Bookmark) string {
	var sb strings.Builder
	const (
		idWithColor    = 16
		minTagsLen     = 34
		defaultTagsLen = 24
	)
	width := terminal.MaxWidth
	idLen := format.PaddingConditional(5, idWithColor)
	tagsLen := format.PaddingConditional(minTagsLen, defaultTagsLen)
	// calculate maximum length for url and tags based on total width
	urlLen := width - idLen - tagsLen
	// define template with formatted placeholders
	template := "%-*s %s %-*s %-*s\n"

	coloredID := color.BrightYellow(b.ID).Bold().String()
	shortURL := format.Shorten(b.URL, urlLen)
	colorURL := color.Gray(shortURL).String()
	urlLen += len(colorURL) - len(shortURL)
	tagsColor := color.BrightCyan(b.Tags).Italic().String()
	result := fmt.Sprintf(
		template,
		idLen,
		coloredID,
		format.UnicodeMidBulletPoint,
		urlLen,
		colorURL,
		tagsLen,
		tagsColor,
	)
	sb.WriteString(result)

	return sb.String()
}

// Multiline formats a bookmark for fzf with max width.
func Multiline(b *Bookmark) string {
	width := terminal.MaxWidth
	var sb strings.Builder
	sb.WriteString(color.BrightYellow(b.ID).Bold().String())
	sb.WriteString(" " + format.UnicodeMidBulletPoint + " ") // sep
	sb.WriteString(format.Shorten(PrettifyURL(b.URL, color.BrightMagenta), width) + "\n")
	sb.WriteString(color.Cyan(format.Shorten(b.Title, width)).String() + "\n")
	sb.WriteString(color.BrightGray(PrettifyTags(b.Tags)).Italic().String())

	return sb.String()
}

func FrameFormatted(b *Bookmark, c color.ColorFn) string {
	f := frame.New(frame.WithColorBorder(c))
	width := terminal.MinWidth
	width -= len(f.Border.Row)
	// split
	descSplit := format.SplitIntoChunks(b.Desc, width)
	titleSplit := format.SplitIntoChunks(b.Title, width)
	// add color and style
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := format.Shorten(PrettifyURL(b.URL, color.BrightMagenta), width)
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.Gray)
	tags := color.Gray(PrettifyTags(b.Tags)).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln().
		Mid(title...).Ln().
		Mid(desc...).Ln().
		Footer(tags).Ln().
		String()
}

// Frame formats a bookmark in a frame with min width.
func Frame(b *Bookmark) string {
	width := terminal.MinWidth
	f := frame.New(frame.WithColorBorder(color.Gray))
	// indentation
	width -= len(f.Border.Row)
	// split and add intendation
	descSplit := format.SplitIntoChunks(b.Desc, width)
	titleSplit := format.SplitIntoChunks(b.Title, width)
	// add color and style
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := format.Shorten(PrettifyURL(b.URL, color.BrightMagenta), width)
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.Gray)
	tags := color.BrightGray(PrettifyTags(b.Tags)).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln().
		Mid(title...).Ln().
		Mid(desc...).Ln().
		Footer(tags).Ln().
		String()
}

// PrettifyTags returns a prettified tags.
func PrettifyTags(s string) string {
	t := strings.ReplaceAll(s, ",", format.UnicodeMidBulletPoint)
	return strings.TrimRight(t, format.UnicodeMidBulletPoint)
}

// PrettifyURL returns a prettified URL.
func PrettifyURL(s string, c color.ColorFn) string {
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

	pathSeg := color.Text(
		format.UnicodeSingleAngleMark,
		strings.Join(pathSegments, fmt.Sprintf(" %s ", format.UnicodeSingleAngleMark)),
	).Italic()

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// FzfFormatter returns a function to format a bookmark for the FZF menu.
func FzfFormatter(m bool) func(b *Bookmark) string {
	if m {
		return Multiline
	}

	return Oneline
}
