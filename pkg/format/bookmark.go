package format

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	spaces      = 6
	newLine     = 1
	indentation = 8
)

var (
	separator = strings.Repeat(" ", spaces) + "+"
)

type Bookmarker interface {
	GetID() int
	GetURL() string
	GetTags() string
	GetTitle() string
	GetDesc() string
	GetCreatedAt() string
}

// Oneline prints a bookmark in a single line
func Oneline(b Bookmarker, color bool, maxWidth int) string {
	var sb strings.Builder
	const maxTagsLen = 18

	maxIDLen := func() int {
		if color {
			return 14
		}
		return 5
	}()

	// calculate maximum length for url and tags based on total width
	maxURLLen := maxWidth - maxIDLen - maxTagsLen

	// define template with formatted placeholders
	template := "%-*s %-*s %-*s\n"

	coloredID := Color(strconv.Itoa(b.GetID())).Yellow().String()
	shortenedURL := ShortenString(b.GetURL(), maxURLLen)
	formattedTags := Color(b.GetTags()).Cyan().String()

	sb.WriteString(fmt.Sprintf(template, maxIDLen, coloredID, maxURLLen, shortenedURL, maxTagsLen, formattedTags))

	return sb.String()
}

// Pretty prints a slice of bookmarks with colors
func Pretty(b Bookmarker, maxWidth int) string {
	var (
		sb      strings.Builder
		maxLine = maxWidth - len(separator) - newLine
		title   = SplitAndAlignString(b.GetTitle(), maxLine, indentation)
		bURL    = ShortenString(b.GetURL(), maxLine)
		desc    = SplitAndAlignString(b.GetDesc(), maxLine, indentation)
	)

	sb.WriteString(HeaderLine(b.GetID(), Color(title).Purple().Bold().String()))
	sb.WriteString(Color(separator, bURL, "\n").Blue().String())
	sb.WriteString(Color(separator, b.GetTags(), "\n").Gray().String())
	sb.WriteString(Color(separator, desc, "\n").String())
	return sb.String()
}

// PrettyWithURLPath prints a bookmark with a URL formatted as a path
//
// Example: www.example.org • search • query
func PrettyWithURLPath(b Bookmarker, maxWidth int) string {
	var (
		sb        strings.Builder
		maxLine   = maxWidth - len(separator) - newLine
		title     = SplitAndAlignString(b.GetTitle(), maxLine, indentation)
		prettyURL = urlPath(b.GetURL())
		bURL      = ShortenString(prettyURL, maxLine)
		desc      = SplitAndAlignString(b.GetDesc(), maxLine, indentation)
	)

	sb.WriteString(HeaderLine(b.GetID(), Color(bURL).Purple().String()))
	sb.WriteString(Color(separator, title, "\n").Blue().String())
	sb.WriteString(Color(separator, prettifyTags(b.GetTags()), "\n").Gray().String())
	sb.WriteString(Color(separator, desc, "\n").String())
	return sb.String()
}

// Delete formats a bookmark for deletion, with the URL displayed in red and
// bold.
func Delete(b Bookmarker, maxWidth int) string {
	var (
		sb        strings.Builder
		maxLine   = maxWidth - len(separator) - newLine
		title     = SplitAndAlignString(b.GetTitle(), maxLine, indentation)
		prettyURL = prettifyURL(b.GetURL())
		bURL      = ShortenString(prettyURL, maxLine)
	)

	sb.WriteString(HeaderLine(b.GetID(), Color(bURL).Red().Bold().String()))
	sb.WriteString(Color(separator, title, "\n").Blue().String())
	sb.WriteString(Color(separator, prettifyTags(b.GetTags()), "\n").Gray().String())
	return sb.String()
}
