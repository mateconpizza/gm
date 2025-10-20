// Package txt provides text formatting helpers.
package txt

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/ui"
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

func PaddedLineWithPad(s, v any, pad int) string {
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

// SplitIntoChunks splits text into lines of maximum length while preserving paragraphs.
func SplitIntoChunks(s string, maxLen int) []string {
	var result []string

	// Split into paragraphs (preserve empty lines)
	paragraphs := strings.Split(s, "\n")

	for i, para := range paragraphs {
		// Empty line = preserve it
		if strings.TrimSpace(para) == "" {
			result = append(result, "")
			continue
		}

		// Wrap the paragraph into lines
		lines := wrapParagraph(para, maxLen)
		result = append(result, lines...)

		// Don't add extra empty line after last paragraph
		if i < len(paragraphs)-1 {
			// Check if next paragraph is also empty (consecutive empty lines)
			if i+1 < len(paragraphs) && strings.TrimSpace(paragraphs[i+1]) == "" {
				continue // The next iteration will handle the empty line
			}
		}
	}

	return result
}

// wrapParagraph wraps a single paragraph (no newlines) into multiple lines.
func wrapParagraph(para string, maxLen int) []string {
	var lines []string
	var currentLine strings.Builder

	words := strings.FieldsSeq(para) // Remove extra spaces within paragraph

	for word := range words {
		// First word in line
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
			continue
		}

		// Check if adding word exceeds max length
		if currentLine.Len()+1+len(word) > maxLen {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		}
	}

	// Add last line
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
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
func URLBreadCrumbsColor(p *color.Palette, s, uc string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}

	if u.Host == "" || u.Path == "" {
		return p.BrightMagentaBold(s)
	}

	host := p.BrightMagentaBold(u.Host)
	pathSegments := strings.FieldsFunc(
		strings.TrimLeft(u.Path, "/"),
		func(r rune) bool { return r == '/' },
	)

	if len(pathSegments) == 0 {
		return host
	}

	segments := strings.Join(pathSegments, fmt.Sprintf(" %s ", uc))
	pathSeg := p.Italic(fmt.Sprintf("%s %s", uc, segments))

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

// TimeWithAgo formats a Unix timestamp as absolute time and relative duration.
//
// YYYY MMM DD HH:MM (N days ago).
func TimeWithAgo(unixTime string) (absolute, relative string) {
	// Parse the Unix timestamp string
	parsedTime, err := time.Parse("20060102150405", unixTime)
	if err != nil {
		panic(err)
	}

	absolute = parsedTime.Local().Format("2006 Jan 02 15:04")
	relative = RelativeTime(parsedTime.Format("20060102-150405"))

	return absolute, relative
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

// TagsWithColorPound returns a prettified tags with #.
//
//	#tag1 #tag2 #tag3
func TagsWithColorPound(c *ui.Console, s string) string {
	p := c.Palette()
	colors := []func(...any) string{
		p.BrightGreenItalic,
		p.BrightMagentaItalic,
		p.BrightCyanItalic,
		p.BrightBlackItalic,
		p.BrightBlueItalic,
		p.BrightWhiteItalic,
		p.BrightGrayItalic,
	}

	tagsSplit := strings.Split(s, ",")
	sort.Strings(tagsSplit)

	var sb strings.Builder
	for _, t := range tagsSplit {
		if t == "" {
			continue
		}

		rc := colors[rand.Intn(len(colors))] //nolint:gosec //unnecessary
		sb.WriteString(fmt.Sprintf("%s%s ", rc("#"), p.Italic(t)))
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
func CenteredLine(width int, label, char string) string {
	const spaces = 2
	if width < len(label)+spaces {
		return label
	}

	dashCount := width - len(label) - spaces
	left := dashCount / 2
	right := dashCount - left

	return strings.Repeat(char, left) + " " + label + " " + strings.Repeat(char, right)
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

// CleanLines removes empty lines and trims whitespace from each line.
func CleanLines(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) == 1 {
		return strings.TrimSpace(s)
	}

	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}

	return strings.Join(result, "\n")
}
