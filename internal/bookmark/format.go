package bookmark

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util/frame"
)

// colorPadding returns the padding for the colorized output.
func colorPadding(minVal, maxVal int) int {
	if terminal.Color {
		return maxVal
	}

	return minVal
}

// Oneline formats a bookmark in a single line.
func FormatOneline(b *Bookmark, maxWidth int) string {
	var sb strings.Builder
	const (
		idWithColor    = 16
		minTagsLen     = 34
		defaultTagsLen = 24
	)

	idLen := colorPadding(5, idWithColor)
	tagsLen := colorPadding(minTagsLen, defaultTagsLen)

	// calculate maximum length for url and tags based on total width
	urlLen := maxWidth - idLen - tagsLen

	// define template with formatted placeholders
	template := "%-*s %s %-*s %-*s\n"

	coloredID := color.BrightYellow(b.GetID()).Bold().String()
	shortURL := format.ShortenString(b.GetURL(), urlLen)
	colorURL := color.BrightWhite(shortURL).String()
	urlLen += len(colorURL) - len(shortURL)
	tagsColor := color.BrightCyan(b.GetTags()).Italic().String()
	result := fmt.Sprintf(
		template,
		idLen,
		coloredID,
		midBulletPoint,
		urlLen,
		colorURL,
		tagsLen,
		tagsColor,
	)
	sb.WriteString(result)

	return sb.String()
}

// Multiline formats a bookmark for fzf.
func Multiline(b *Bookmark, maxWidth int) string {
	n := maxWidth
	var sb strings.Builder

	id := color.BrightYellow(b.GetID()).Bold().String()
	url := color.BrightMagenta(format.ShortenString(PrettifyURL(b.GetURL()), n)).
		String()
	title := format.ShortenString(b.GetTitle(), n)
	tags := color.Gray(PrettifyTags(b.GetTags())).Italic().String()

	sb.WriteString(fmt.Sprintf("%s %s %s\n%s\n%s", id, midBulletPoint, url, title, tags))

	return sb.String()
}

// PrettyWithURLPath formats a bookmark with a URL formatted as a path
//
// Example: www.example.org • search • query.
func PrettyWithURLPath(b *Bookmark, maxWidth int) string {
	const (
		bulletPoint = "\u2022" // •
		indentation = 8
		newLine     = 2
		spaces      = 6
	)

	var (
		sb        strings.Builder
		separator = strings.Repeat(" ", spaces) + "+"
		maxLine   = maxWidth - len(separator) - newLine
		title     = format.SplitAndAlignLines(b.GetTitle(), maxLine, indentation)
		prettyURL = PrettifyURLPath(b.GetURL())
		shortURL  = format.ShortenString(prettyURL, maxLine)
		desc      = format.SplitAndAlignLines(b.GetDesc(), maxLine, indentation)
		id        = color.BrightWhite(b.GetID()).String()
		idSpace   = len(separator) - 1
		idPadding = strings.Repeat(" ", idSpace-len(strconv.Itoa(b.GetID())))
	)

	// Construct the formatted string
	sb.WriteString(
		fmt.Sprintf("%s%s%s %s\n", id, idPadding, bulletPoint, color.Purple(shortURL).String()),
	)
	sb.WriteString(color.Cyan(separator, title, "\n").String())
	sb.WriteString(color.Gray(separator, PrettifyTags(b.GetTags()), "\n").Italic().String())
	sb.WriteString(color.BrightWhite(separator, desc).String())

	return sb.String()
}

// WithFrameAndURLColor formats a bookmark with a given color.
func WithFrameAndURLColor(
	f *frame.Frame,
	b *Bookmark,
	n int,
	c func(arg ...any) *color.Color,
) {
	const _midBulletPoint = "\u00b7"
	n -= len(f.Border.Row)

	titleSplit := format.SplitIntoLines(b.GetTitle(), n)
	idStr := color.BrightWhite(b.GetID()).Bold().String()

	url := c(format.ShortenString(PrettifyURL(b.GetURL()), n)).String()
	title := color.ApplyMany(titleSplit, color.Cyan)
	tags := color.Gray(PrettifyTags(b.GetTags())).Italic().String()

	f.Mid(fmt.Sprintf("%s %s %s", idStr, _midBulletPoint, url))
	f.Mid(title...).Mid(tags).Newline()
}

// FormatWithFrame formats a bookmark in a frame.
func FormatWithFrame(b *Bookmark, maxWidth int) string {
	n := maxWidth
	f := frame.New(
		frame.WithColorBorder(color.Gray),
		frame.WithMaxWidth(n),
	)

	// Indentation
	n -= len(f.Border.Row)

	// Split and add intendation
	descSplit := format.SplitIntoLines(b.GetDesc(), n)
	titleSplit := format.SplitIntoLines(b.GetTitle(), n)

	// Add color and style
	id := color.BrightYellow(b.GetID()).Bold().String()
	url := color.BrightMagenta(format.ShortenString(PrettifyURL(b.GetURL()), n)).
		String()
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.BrightWhite)
	tags := color.Gray(PrettifyTags(b.GetTags())).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, url)).
		Mid(title...).Mid(desc...).
		Footer(tags).String()
}

// FormatBuffer returns a complete buf.
func FormatBuffer(b *Bookmark) []byte {
	return []byte(fmt.Sprintf(`# URL:
%s
# Title: (leave an empty line for web fetch)
%s
# Tags: (comma separated)
%s
# Description: (leave an empty line for web fetch)
%s
# end
`, b.GetURL(), b.GetTitle(), b.GetTags(), b.GetDesc()))
}

// formatBufferSimple returns a simple buf with ID, title, tags and URL.
func formatBufferSimple(b *Bookmark) []byte {
	id := fmt.Sprintf("[%d]", b.ID)
	return []byte(fmt.Sprintf("# %s %10s\n# tags: %s\n%s\n\n", id, b.Title, b.Tags, b.URL))
}
