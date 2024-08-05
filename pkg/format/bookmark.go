package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haaag/gm/pkg/format/color"
)

const (
	_spaces      int = 6
	_newLine     int = 2
	_indentation int = 8
)

var _separator string = strings.Repeat(" ", _spaces) + "+"

type Bookmarker interface {
	GetID() int
	GetURL() string
	GetTags() string
	GetTitle() string
	GetDesc() string
	GetCreatedAt() string
}

// Oneline prints a bookmark in a single line.
func Oneline(b Bookmarker, hasColor bool, maxWidth int) string {
	var (
		maxTagsLen = 18
		sb         strings.Builder
		withColor  = 14
	)

	maxIDLen := func() int {
		if hasColor {
			return withColor
		}

		return 5
	}()

	// calculate maximum length for url and tags based on total width
	maxURLLen := maxWidth - maxIDLen - maxTagsLen

	// define template with formatted placeholders
	template := "%-*s %-*s %-*s\n"

	coloredID := color.BrightYellow(strconv.Itoa(b.GetID())).String()
	shortenedURL := ShortenString(b.GetURL(), maxURLLen)
	colorURL := color.BrightWhite(shortenedURL).String()
	maxURLLen += len(colorURL) - len(shortenedURL)
	formattedTags := color.BrightCyan(b.GetTags()).Italic().String()
	sb.WriteString(
		fmt.Sprintf(template, maxIDLen, coloredID, maxURLLen, colorURL, maxTagsLen, formattedTags),
	)

	return sb.String()
}

// Pretty formats a slice of bookmarks with colors.
func Pretty(b Bookmarker, maxWidth int) string {
	var (
		sb      strings.Builder
		maxLine = maxWidth - len(_separator) - _newLine
		title   = SplitAndAlignString(b.GetTitle(), maxLine, _indentation)
		bURL    = ShortenString(b.GetURL(), maxLine)
		desc    = SplitAndAlignString(b.GetDesc(), maxLine, _indentation)
		id      = color.BrightWhite(strconv.Itoa(b.GetID())).Bold().String()
	)

	idSpace := 6
	n := len(strconv.Itoa(b.GetID()))
	_sep := strings.Repeat(" ", idSpace-n)
	sb.WriteString(
		fmt.Sprintf("%s%s%s %s\n", id, _sep, _bulletPoint, color.Orange(bURL).String()),
	)
	sb.WriteString(color.Cyan(_separator, title, "\n").String())
	sb.WriteString(color.Gray(_separator, b.GetTags(), "\n").String())
	sb.WriteString(color.BrightWhite(_separator, desc, "\n").String())

	return sb.String()
}

// PrettyWithURLPath formats a bookmark with a URL formatted as a path
//
// Example: www.example.org • search • query.
func PrettyWithURLPath(b Bookmarker, maxWidth int) string {
	var (
		sb        strings.Builder
		maxLine   = maxWidth - len(_separator) - _newLine
		title     = SplitAndAlignString(b.GetTitle(), maxLine, _indentation)
		prettyURL = urlPath(b.GetURL())
		bURL      = ShortenString(prettyURL, maxLine)
		desc      = SplitAndAlignString(b.GetDesc(), maxLine, _indentation)
		id        = color.BrightWhite(strconv.Itoa(b.GetID())).String()
	)

	idSpace := 6
	n := len(strconv.Itoa(b.GetID()))
	_sep := strings.Repeat(" ", idSpace-n)
	sb.WriteString(
		fmt.Sprintf("%s%s%s %s\n", id, _sep, _bulletPoint, color.Purple(bURL).String()),
	)
	sb.WriteString(color.Cyan(_separator, title, "\n").String())
	sb.WriteString(color.Gray(_separator, prettifyTags(b.GetTags()), "\n").Italic().String())
	sb.WriteString(color.BrightWhite(_separator, desc).String())

	return sb.String()
}

// ColorWithURLPath formats a bookmark with a given color.
func ColorWithURLPath(b Bookmarker, maxWidth int, colors func(...string) *color.Color) string {
	var (
		sb        strings.Builder
		maxLine   = maxWidth - len(_separator) - _newLine
		title     = SplitAndAlignString(b.GetTitle(), maxLine, _indentation)
		prettyURL = prettifyURL(b.GetURL())
		bURL      = ShortenString(prettyURL, maxLine)
	)

	sb.WriteString(headerIDLine(b.GetID(), colors(bURL).Bold().String()))
	sb.WriteString(color.Blue(_separator, title, "\n").String())
	sb.WriteString(color.Gray(_separator, prettifyTags(b.GetTags()), "\n").String())

	return sb.String()
}
