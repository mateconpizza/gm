package bookmark

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/slice"
)

// Oneline formats a bookmark in a single line.
func FormatOneline(b *Bookmark, maxWidth int) string {
	var sb strings.Builder
	const (
		idWithColor    = 16
		minTagsLen     = 34
		defaultTagsLen = 24
	)

	idLen := format.PaddingConditional(5, idWithColor)
	tagsLen := format.PaddingConditional(minTagsLen, defaultTagsLen)

	// calculate maximum length for url and tags based on total width
	urlLen := maxWidth - idLen - tagsLen

	// define template with formatted placeholders
	template := "%-*s %s %-*s %-*s\n"

	coloredID := color.BrightYellow(b.ID).Bold().String()
	shortURL := format.ShortenString(b.URL, urlLen)
	colorURL := color.BrightWhite(shortURL).String()
	urlLen += len(colorURL) - len(shortURL)
	tagsColor := color.BrightCyan(b.Tags).Italic().String()
	result := fmt.Sprintf(
		template,
		idLen,
		coloredID,
		format.MidBulletPoint,
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

	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := color.BrightMagenta(format.ShortenString(PrettifyURL(b.URL), n)).
		String()
	title := format.ShortenString(b.Title, n)
	tags := color.Gray(PrettifyTags(b.Tags)).Italic().String()

	sb.WriteString(
		fmt.Sprintf("%s %s %s\n%s\n%s", id, format.MidBulletPoint, urlColor, title, tags),
	)

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
		title     = format.SplitAndAlignLines(b.Title, maxLine, indentation)
		prettyURL = PrettifyURLPath(b.URL)
		shortURL  = format.ShortenString(prettyURL, maxLine)
		desc      = format.SplitAndAlignLines(b.Desc, maxLine, indentation)
		id        = color.BrightWhite(b.ID).String()
		idSpace   = len(separator) - 1
		idPadding = strings.Repeat(" ", idSpace-len(strconv.Itoa(b.ID)))
	)

	// Construct the formatted string
	sb.WriteString(
		fmt.Sprintf("%s%s%s %s\n", id, idPadding, bulletPoint, color.Purple(shortURL).String()),
	)
	sb.WriteString(color.Cyan(separator, title, "\n").String())
	sb.WriteString(color.Gray(separator, PrettifyTags(b.Tags), "\n").Italic().String())
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
	n -= len(f.Border.Row)

	titleSplit := format.SplitIntoLines(b.Title, n)
	idStr := color.BrightWhite(b.ID).Bold().String()

	urlColor := c(format.ShortenString(PrettifyURL(b.URL), n)).String()
	title := color.ApplyMany(titleSplit, color.Cyan)
	tags := color.Gray(PrettifyTags(b.Tags)).Italic().String()

	f.Mid(fmt.Sprintf("%s %s %s", idStr, format.MidBulletPoint, urlColor))
	f.Mid(title...).Mid(tags).Newline()
}

// FormatWithFrame formats a bookmark in a frame.
func FormatWithFrame(b *Bookmark, maxWidth int) string {
	n := maxWidth
	f := frame.New(frame.WithColorBorder(color.Gray))

	// Indentation
	n -= len(f.Border.Row)

	// Split and add intendation
	descSplit := format.SplitIntoLines(b.Desc, n)
	titleSplit := format.SplitIntoLines(b.Title, n)

	// Add color and style
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := color.BrightMagenta(format.ShortenString(PrettifyURL(b.URL), n)).
		String()
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.BrightWhite)
	tags := color.Gray(PrettifyTags(b.Tags)).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, urlColor)).
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
`, b.URL, b.Title, b.Tags, b.Desc))
}

// PrettifyTags returns a prettified tags.
func PrettifyTags(s string) string {
	t := strings.ReplaceAll(s, ",", format.MidBulletPoint)
	return strings.TrimRight(t, format.MidBulletPoint)
}

// PrettifyURLPath returns a prettified URL.
func PrettifyURLPath(bURL string) string {
	u, err := url.Parse(bURL)
	if err != nil {
		return ""
	}

	if u.Host == "" || u.Path == "" {
		return color.Text(bURL).Bold().String()
	}

	host := color.Text(u.Host).Bold().String()
	pathSegments := strings.FieldsFunc(
		strings.TrimLeft(u.Path, "/"),
		func(r rune) bool { return r == '/' },
	)

	if len(pathSegments) == 0 {
		return host
	}

	pathSeg := color.Gray(
		format.PathSmallSegment,
		strings.Join(pathSegments, fmt.Sprintf(" %s ", format.PathSmallSegment)),
	)

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// PrettifyURL returns a prettified URL.
func PrettifyURL(bURL string) string {
	u, err := url.Parse(bURL)
	if err != nil {
		return ""
	}

	if u.Host == "" || u.Path == "" {
		return color.Text(bURL).Bold().String()
	}

	host := color.Text(u.Host).Bold().String()
	pathSegments := strings.FieldsFunc(
		strings.TrimLeft(u.Path, "/"),
		func(r rune) bool { return r == '/' },
	)

	if len(pathSegments) == 0 {
		return host
	}

	pathSeg := color.Gray(
		format.PathSmallSegment,
		strings.Join(pathSegments, fmt.Sprintf(" %s ", format.PathSmallSegment)),
	).Italic()

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// GetBufferSlice returns a buffer with the provided slice of bookmarks.
func GetBufferSlice(bs *slice.Slice[Bookmark]) []byte {
	// FIX: replace with menu
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("## Remove the <URL> line to ignore bookmark\n")
	fmt.Fprintf(buf, "## Showing %d bookmark/s\n\n", bs.Len())
	bs.ForEach(func(b Bookmark) {
		buf.Write(formatBufferSimple(&b))
	})

	return bytes.TrimSpace(buf.Bytes())
}

// formatBufferSimple returns a simple buf with ID, title, tags and URL.
func formatBufferSimple(b *Bookmark) []byte {
	id := fmt.Sprintf("[%d]", b.ID)
	return []byte(fmt.Sprintf("# %s %10s\n# tags: %s\n%s\n\n", id, b.Title, b.Tags, b.URL))
}
