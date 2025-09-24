// Package txt provides text formatting helpers.
package txt

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/ui/color"
)

const (
	UnicodeBulletPoint      = "\u2022" // •
	UnicodeLightDiagCross   = "\u2571" // ╱
	UnicodeMiddleDot        = "\u00b7" // ·
	UnicodePathBigSegment   = "\u25B6" // ▶
	UnicodePathSmallSegment = "\u25B8" // ▸
	UnicodeRightDoubleAngle = "\u00BB" // »
	UnicodeSingleAngleMark  = "\u203A" // ›
	UnicodeNotes            = "\U0001F4DD"
)

// TimeLayout is the default layout for time formatting.
const TimeLayout = "20060102-150405"

// NBSP represents a non-breaking space character.
const NBSP = "\u00A0"

// spaces returns a string with n spaces.
func spaces(n int) string {
	return fmt.Sprintf("%*s", n, "")
}

// PaddedLine formats a label and value into a left-aligned bullet point with fixed padding.
func PaddedLine(s, v any) string {
	const pad = 15

	str := fmt.Sprint(s)
	visibleLen := len(color.ANSICodeRemover(str))
	padding := max(pad-visibleLen, 0)

	return fmt.Sprintf("%s%s %v", str, spaces(padding), v)
}

// Shorten shortens a string to a maximum length.
//
//	string...
func Shorten(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}

	return s
}

// SplitAndAlign splits a string into multiple lines and aligns the
// words.
func SplitAndAlign(s string, lineLength, indentation int) string {
	var (
		result      strings.Builder
		currentLine strings.Builder
	)

	separator := strings.Repeat(" ", indentation)

	for word := range strings.FieldsSeq(s) {
		if currentLine.Len()+len(word)+1 > lineLength {
			result.WriteString(currentLine.String())
			result.WriteString("\n")
			currentLine.Reset()
			currentLine.WriteString(separator)
			currentLine.WriteString(word)
		} else {
			if currentLine.Len() != 0 {
				currentLine.WriteString(" ")
			}

			currentLine.WriteString(word)
		}
	}

	result.WriteString(currentLine.String())

	return result.String()
}

// SplitIntoChunks splits strings lines into chunks of a given length.
func SplitIntoChunks(s string, strLen int) []string {
	var (
		lines       []string
		currentLine strings.Builder
	)

	// Remember if the original string ended with a newline.
	endsWithNewline := strings.HasSuffix(s, "\n")

	for word := range strings.FieldsSeq(s) {
		// If currentLine is empty, write the word directly.
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
			continue
		}
		// Check if adding the new word would exceed the line length
		if currentLine.Len()+len(word)+1 > strLen {
			// Add the current line to the result and start a new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			// Add the word to the current line
			if currentLine.Len() != 0 {
				currentLine.WriteString(" ")
			}

			currentLine.WriteString(word)
		}
	}

	// Add the last line if it's not empty
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	// If the original string ended with a newline, add an extra empty string.
	if endsWithNewline {
		lines = append(lines, "")
	}

	return lines
}

// NormalizeSpace removes extra whitespace from a string, leaving only single
// spaces between words.
func NormalizeSpace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// URLBreadCrumbs returns a prettified URL with color.
//
//	https://example.org/title/some-title
//	https://example.org > title > some-title
func URLBreadCrumbs(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if u.Host == "" || u.Path == "" {
		return s
	}

	host := u.Host
	pathSegments := strings.FieldsFunc(
		strings.TrimLeft(u.Path, "/"),
		func(r rune) bool { return r == '/' },
	)

	if len(pathSegments) == 0 {
		return host
	}

	uc := UnicodeSingleAngleMark
	segments := strings.Join(pathSegments, fmt.Sprintf(" %s ", uc))
	pathSeg := uc + " " + segments

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// URLBreadCrumbsColor returns a prettified URL with color.
//
//	https://example.org/title/some-title
//	https://example.org > title > some-title
func URLBreadCrumbsColor(s string, c color.ColorFn) string {
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

	uc := UnicodeSingleAngleMark
	segments := strings.Join(pathSegments, fmt.Sprintf(" %s ", uc))
	pathSeg := color.Text(uc, segments).Italic()

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// CountLines counts the number of lines in a string.
func CountLines(s string) int {
	return len(strings.Split(s, "\n"))
}

// RelativeTime takes a timestamp string in the format "20060102-150405"
// and returns a relative description.
//
//	"today", "yesterday" or "X days ago"
func RelativeTime(ts string) string {
	t, err := time.Parse(TimeLayout, ts)
	if err != nil {
		return "invalid timestamp"
	}

	now := time.Now()

	// calculate the duration between now and the timestamp.
	// we assume the timestamp is in the past.
	diff := now.Sub(t)
	days := int(diff.Hours() / 24)
	if days <= 0 {
		return "today"
	}
	if days == 1 {
		return "yesterday"
	}

	return fmt.Sprintf("%d days ago", days)
}

// RelativeISOTime takes a timestamp string in ISO 8601 format (e.g., "2025-02-27T05:03:28Z")
// and returns a relative time description.
func RelativeISOTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "invalid timestamp"
	}

	now := time.Now()
	// Normalize to local date only (ignore hour/minute/second)
	t = t.Local()
	now = now.Local()

	// Zero the time component for day comparison
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	diff := now.Sub(t)
	days := int(diff.Hours() / 24)

	switch {
	case days < 0:
		return "in the future"
	case days == 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days < 7:
		return fmt.Sprintf("%d days ago", days)
	case days < 14:
		return "1 week ago"
	case days < 28:
		return fmt.Sprintf("%d weeks ago", days/7)
	case days < 60:
		return "1 month ago"
	case days < 365:
		return fmt.Sprintf("%d months ago", days/30)
	case days < 730:
		return "1 year ago"
	default:
		return fmt.Sprintf("%d years ago", days/365)
	}
}

// TagsWithPound returns a prettified tags with #.
//
//	#tag1 #tag2 #tag3
func TagsWithPound(s string) string {
	var sb strings.Builder

	tagsSplit := strings.Split(s, ",")
	sort.Strings(tagsSplit)

	for _, t := range tagsSplit {
		if t == "" {
			continue
		}

		sb.WriteString(fmt.Sprintf("#%s ", t))
	}

	return sb.String()
}

// TagsWithPoundList returns a prettified tags list with #.
func TagsWithPoundList(s string) []string {
	return strings.FieldsFunc(TagsWithPound(s), func(r rune) bool { return r == ' ' })
}

// TagsWithUnicode returns a prettified tags.
//
//	tag1·tag2·tag3
func TagsWithUnicode(s string) string {
	ud := UnicodeMiddleDot
	return strings.TrimRight(strings.ReplaceAll(s, ",", ud), ud)
}

// CenteredLine returns a string of exactly 'width' characters,
// centering the label between dashes.
//
//	-------- label --------
func CenteredLine(width int, label string) string {
	const spaces = 2
	if width < len(label)+spaces {
		return label
	}

	dashCount := width - len(label) - spaces
	left := dashCount / 2
	right := dashCount - left

	return strings.Repeat("-", left) + " " + label + " " + strings.Repeat("-", right)
}

// GenHash generates a hash from a string with the given length.
func GenHash(s string, c int) string {
	hash := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(hash[:])[:c]
}

// GenHashPath generates a hash from a full path.
func GenHashPath(fullPath string) string {
	hash := sha256.Sum256([]byte(fullPath))
	return hex.EncodeToString(hash[:])
}

// CreateSimpleTable generates a simple ASCII table with basic borders.
func CreateSimpleTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Calculate column widths using ANSI-stripped content
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(color.ANSICodeRemover(header))
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(color.ANSICodeRemover(cell)) > colWidths[i] {
				colWidths[i] = len(color.ANSICodeRemover(cell))
			}
		}
	}

	var builder strings.Builder

	// Create top border
	builder.WriteString("+")
	for _, width := range colWidths {
		builder.WriteString(strings.Repeat("-", width+2) + "+")
	}
	builder.WriteString("\n")

	// Create header row
	builder.WriteString("|")
	for i, header := range headers {
		visibleLen := len(color.ANSICodeRemover(header))
		padding := colWidths[i] - visibleLen
		builder.WriteString(" " + header + strings.Repeat(" ", padding) + " |")
	}
	builder.WriteString("\n")

	// Create header separator
	builder.WriteString("+")
	for _, width := range colWidths {
		builder.WriteString(strings.Repeat("-", width+2) + "+")
	}
	builder.WriteString("\n")

	// Create data rows
	for _, row := range rows {
		builder.WriteString("|")
		for i, width := range colWidths {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			visibleLen := len(color.ANSICodeRemover(cell))
			padding := width - visibleLen
			builder.WriteString(" " + cell + strings.Repeat(" ", padding) + " |")
		}
		builder.WriteString("\n")
	}

	// Create bottom border
	builder.WriteString("+")
	for _, width := range colWidths {
		builder.WriteString(strings.Repeat("-", width+2) + "+")
	}
	builder.WriteString("\n")

	return builder.String()
}
