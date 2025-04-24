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

// Oneline formats a bookmark in a single line with the given colorscheme.
func Oneline(b *Bookmark, cs *color.Scheme) string {
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
	sb.WriteString(cs.Cyan(format.Shorten(b.Title, w)).String() + "\n")
	sb.WriteString(cs.BrightWhite(format.TagsWithUnicode(b.Tags)).Italic().String())

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
func Frame(b *Bookmark, cs *color.Scheme) string {
	w := terminal.MinWidth
	f := frame.New(frame.WithColorBorder(cs.BrightBlack))
	// indentation
	w -= len(f.Border.Row)
	// split and add indentation
	descSplit := format.SplitIntoChunks(b.Desc, w)
	titleSplit := format.SplitIntoChunks(b.Title, w)
	// add color and style
	id := cs.BrightYellow(b.ID).Bold()
	urlColor := format.Shorten(format.URLBreadCrumbs(b.URL, cs.BrightMagenta), w)
	title := color.ApplyMany(titleSplit, cs.BrightCyan)
	desc := color.ApplyMany(descSplit, cs.White)
	tags := cs.BrightWhite(format.TagsWithPound(b.Tags)).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, urlColor)).Ln().
		Mid(title...).Ln().
		Mid(desc...).Ln().
		Footer(tags).Ln().
		String()
}

// prettifyTags returns a prettified tags.
//
//	tag1·tag2·tag3
func prettifyTags(s string) string {
	uc := format.UnicodeMiddleDot
	return strings.TrimRight(strings.ReplaceAll(s, ",", uc), uc)
}

// breadcrumbsURL returns a prettified URL with color.
//
//	https://example.org/title/some-title
//	https://example.org > title > some-title
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
func FzfFormatter(m bool, colorScheme string) func(b *Bookmark) string {
	var (
		cs *color.Scheme
		ok bool
	)
	cs, ok = color.DefaultSchemes[colorScheme]
	if !ok {
		cs = color.DefaultColorScheme()
	}

	switch {
	case m:
		return func(b *Bookmark) string {
			return Multiline(b, cs)
		}
	default:
		return func(b *Bookmark) string {
			return Oneline(b, cs)
		}
	}
}
