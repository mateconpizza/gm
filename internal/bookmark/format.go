package bookmark

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/terminal"
)

// Buffer returns a formatted buffer with item attrs.
func Buffer(b *Bookmark) []byte {
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

// BufferSlice returns a buffer with the provided slice of bookmarks.
func BufferSlice(bs *slice.Slice[Bookmark]) []byte {
	// FIX: replace with menu
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("## Remove the <URL> line to ignore bookmark\n")
	fmt.Fprintf(buf, "## Showing %d bookmark/s\n\n", bs.Len())
	bs.ForEach(func(b Bookmark) {
		buf.Write(formatBufferSimple(&b))
	})

	return bytes.TrimSpace(buf.Bytes())
}

// Oneline formats a bookmark in a single line.
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
func Multiline(b *Bookmark) string {
	width := terminal.MaxWidth
	var sb strings.Builder
	sb.WriteString(color.BrightYellow(b.ID).Bold().String())
	sb.WriteString(" " + format.MidBulletPoint + " ") // sep
	sb.WriteString(format.Shorten(PrettifyURL(b.URL, color.BrightMagenta), width) + "\n")
	sb.WriteString(color.Cyan(format.Shorten(b.Title, width)).String() + "\n")
	sb.WriteString(color.BrightGray(PrettifyTags(b.Tags)).Italic().String())

	return sb.String()
}

func FrameFormatted(b *Bookmark, width int, c color.ColorFn) string {
	f := frame.New(frame.WithColorBorder(c))
	width -= len(f.Border.Row)

	// split and add intendation
	descSplit := format.Split(b.Desc, width)
	titleSplit := format.Split(b.Title, width)

	// add color and style
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := format.Shorten(PrettifyURL(b.URL, color.BrightMagenta), width)
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.Gray)
	tags := color.Gray(PrettifyTags(b.Tags)).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, urlColor)).
		Mid(title...).
		Mid(desc...).
		Footer(tags).
		String()
}

// FmtWithFrame formats and displays a bookmark with styling and frame layout.
func FmtWithFrame(f *frame.Frame, b *Bookmark, n int, c color.ColorFn) {
	n -= len(f.Border.Row)

	titleSplit := format.Split(b.Title, n)
	idStr := color.BrightWhite(b.ID).Bold().String()

	urlColor := format.Shorten(PrettifyURL(b.URL, c), n)
	title := color.ApplyMany(titleSplit, color.Cyan)
	tags := color.Gray(PrettifyTags(b.Tags)).Italic().String()

	f.Mid(fmt.Sprintf("%s %s %s", idStr, format.MidBulletPoint, urlColor)).Ln()
	f.Mid(title...).Ln()
	f.Mid(tags).Ln()
}

// Frame formats a bookmark in a frame.
func Frame(b *Bookmark, width int) string {
	f := frame.New(frame.WithColorBorder(color.Gray))

	// indentation
	width -= len(f.Border.Row)

	// split and add intendation
	descSplit := format.Split(b.Desc, width)
	titleSplit := format.Split(b.Title, width)

	// add color and style
	id := color.BrightYellow(b.ID).Bold().String()
	urlColor := format.Shorten(PrettifyURL(b.URL, color.BrightMagenta), width)
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.Gray)
	tags := color.BrightGray(PrettifyTags(b.Tags)).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, urlColor)).
		Mid(title...).
		Mid(desc...).
		Footer(tags).
		String()
}

// PrettifyTags returns a prettified tags.
func PrettifyTags(s string) string {
	t := strings.ReplaceAll(s, ",", format.MidBulletPoint)
	return strings.TrimRight(t, format.MidBulletPoint)
}

// PrettifyURL returns a prettified URL.
func PrettifyURL(s string, c color.ColorFn) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}

	if c == nil {
		c = color.BrightWhite
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
		format.SingleAngleMark,
		strings.Join(pathSegments, fmt.Sprintf(" %s ", format.SingleAngleMark)),
	).Italic()

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// formatBufferSimple returns a simple buf with ID, title, tags and URL.
func formatBufferSimple(b *Bookmark) []byte {
	// FIX: replace with menu
	id := fmt.Sprintf("[%d]", b.ID)
	return []byte(fmt.Sprintf("# %s %10s\n# tags: %s\n%s\n\n", id, b.Title, b.Tags, b.URL))
}
