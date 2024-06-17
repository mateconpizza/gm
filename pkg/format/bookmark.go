package format

import (
	"fmt"
	"strconv"
	"strings"
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

// Oneline prints a bookmark in a single line
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

	coloredID := Color(strconv.Itoa(b.GetID())).Yellow().String()
	shortenedURL := ShortenString(b.GetURL(), maxURLLen)
	colorURL := Color(shortenedURL).Gray().String()
	maxURLLen += len(colorURL) - len(shortenedURL)
	formattedTags := Color(b.GetTags()).Cyan().String()
	sb.WriteString(fmt.Sprintf(template, maxIDLen, coloredID, maxURLLen, colorURL, maxTagsLen, formattedTags))
	return sb.String()
}

// Pretty formats a slice of bookmarks with colors
func Pretty(b Bookmarker, maxWidth int) string {
	var (
		sb      strings.Builder
		maxLine = maxWidth - len(_separator) - _newLine
		title   = SplitAndAlignString(b.GetTitle(), maxLine, _indentation)
		bURL    = ShortenString(b.GetURL(), maxLine)
		desc    = SplitAndAlignString(b.GetDesc(), maxLine, _indentation)
	)

	sb.WriteString(headerIDLine(b.GetID(), Color(title).Purple().Bold().String()))
	sb.WriteString(Color(_separator, bURL, "\n").Blue().String())
	sb.WriteString(Color(_separator, b.GetTags(), "\n").Gray().String())
	sb.WriteString(Color(_separator, desc, "\n").String())
	return sb.String()
}

// PrettyWithURLPath formats a bookmark with a URL formatted as a path
//
// Example: www.example.org • search • query
func PrettyWithURLPath(b Bookmarker, maxWidth int) string {
	var (
		sb        strings.Builder
		maxLine   = maxWidth - len(_separator) - _newLine
		title     = SplitAndAlignString(b.GetTitle(), maxLine, _indentation)
		prettyURL = urlPath(b.GetURL())
		bURL      = ShortenString(prettyURL, maxLine)
		desc      = SplitAndAlignString(b.GetDesc(), maxLine, _indentation)
	)

	sb.WriteString(headerIDLine(b.GetID(), Color(bURL).Purple().String()))
	sb.WriteString(Color(_separator, title, "\n").Blue().String())
	sb.WriteString(Color(_separator, prettifyTags(b.GetTags()), "\n").Gray().String())
	sb.WriteString(Color(_separator, desc, "\n").String())
	return sb.String()
}

// Delete formats a bookmark for deletion, with the URL displayed in red and
// bold.
func Delete(b Bookmarker, maxWidth int) string {
	var (
		sb        strings.Builder
		maxLine   = maxWidth - len(_separator) - _newLine
		title     = SplitAndAlignString(b.GetTitle(), maxLine, _indentation)
		prettyURL = prettifyURL(b.GetURL())
		bURL      = ShortenString(prettyURL, maxLine)
	)

	sb.WriteString(headerIDLine(b.GetID(), Color(bURL).Red().Bold().String()))
	sb.WriteString(Color(_separator, title, "\n").Blue().String())
	sb.WriteString(Color(_separator, prettifyTags(b.GetTags()), "\n").Gray().String())
	return sb.String()
}
