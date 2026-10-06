// Package txt provides text formatting helpers.
package txt

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	runewidth "github.com/mattn/go-runewidth"

	"github.com/mateconpizza/gm/pkg/ansi"
)

// TimeLayout is the default layout for time formatting.
const TimeLayout = "20060102-150405"

// NBSP represents a non-breaking space character.
const NBSP = "\u00A0"

// PadRight pads s with spaces to width w, measured on visible width.
func PadRight(s string, w int) string {
	if n := w - VisibleWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// PadLeft pads s with spaces to width w, measured on visible width.
func PadLeft(s string, w int) string {
	if n := w - VisibleWidth(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

// PaddedLine formats a label and value into a left-aligned bullet point with
// fixed padding.
func PaddedLine(s, v any) string { return PaddedLineWithWidth(s, v, 14) }

func PaddedLineWithWidth(s, v any, w int) string {
	str := fmt.Sprint(s)
	return fmt.Sprintf("%s %v", PadRight(str, w), v)
}

func PaddedLineWithPadChar(s, v any, w int, padChar string) string {
	str := fmt.Sprint(s)
	padding := max(w-VisibleWidth(str), 0)

	return fmt.Sprintf(
		"%s%s%v",
		str,
		strings.Repeat(padChar, padding),
		v,
	)
}

// Shorten shortens a string to a maximum length.
//
//	string...
//
// Shorten shortens a string to a maximum visual width.
func Shorten(s string, maxWidth int) string {
	return runewidth.Truncate(s, maxWidth, "…")
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
			result.WriteByte('\n')
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

func VisibleWidth(s string) int        { return runewidth.StringWidth(ansi.Remover(s)) }
func StringWidth(s string) int         { return runewidth.StringWidth(s) }
func FillRight(s string, w int) string { return runewidth.FillRight(s, w) }
func FillLeft(s string, w int) string  { return runewidth.FillLeft(s, w) }

// NormalizeSpace removes extra whitespace from a string, leaving only single
// spaces between words.
func NormalizeSpace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// URLBreadCrumbsColor returns a prettified URL with color.
//
//	https://example.org/title/some-title
//	https://example.org > title > some-title
func URLBreadCrumbsColor(p *ansi.Palette, s, uc string, width int) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}

	// Fallback if no host/path
	if u.Host == "" || u.Path == "" {
		// If the whole raw URL is too long, shorten it
		if runewidth.StringWidth(s) > width {
			return p.BrightMagenta.With(p.Bold).Sprint(Shorten(s, width))
		}
		return p.BrightMagenta.With(p.Bold).Sprint(s)
	}

	// Measure Host width (stripped of ANSI)
	hostRaw := u.Host
	hostWidth := runewidth.StringWidth(hostRaw)

	host := p.BrightMagenta.With(p.Bold).Sprint(hostRaw)

	// If Host alone takes up all space (or more), return just shortened host
	if hostWidth >= width {
		return p.BrightMagenta.With(p.Bold).Sprint(Shorten(hostRaw, width))
	}

	pathSegments := strings.FieldsFunc(
		strings.TrimLeft(u.Path, "/"),
		func(r rune) bool { return r == '/' },
	)

	if len(pathSegments) == 0 {
		return host
	}

	// Separator logic
	ucStyled := p.Dim.Sprint(uc)
	sep := fmt.Sprintf(" %s ", uc)
	sepStyled := fmt.Sprintf(" %s ", ucStyled)
	sepWidth := runewidth.StringWidth(sep) // usually 3: " > "

	// Available width for path = Total - Host - Separator
	pathWidth := width - hostWidth - sepWidth

	// Safety check if we ran out of space
	if pathWidth <= 0 {
		return host
	}

	segments := strings.Join(pathSegments, sep)

	// Shorten the raw segments string to fit pathWidth
	shortSegments := Shorten(segments, pathWidth)

	// Replace raw separators with styled ones
	styledSegments := strings.ReplaceAll(shortSegments, sep, sepStyled)
	pathSeg := p.Italic.Sprint(styledSegments)
	return fmt.Sprintf("%s%s", host, sepStyled+pathSeg)
}

// CountLines counts the number of lines in a string.
func CountLines(s string) int { return len(strings.Split(s, "\n")) }

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
	t = t.UTC()
	now = now.UTC()

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
func TimeWithAgo(unixTime string) (absolute, relative string, err error) {
	// Parse the Unix timestamp string
	parsedTime, err := time.Parse("20060102150405", unixTime)
	if err != nil {
		return "", "", err
	}

	absolute = parsedTime.UTC().Format("2006 Jan 02 15:04")
	relative = RelativeTime(parsedTime.Format(TimeLayout))

	return absolute, relative, err
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

		fmt.Fprintf(&sb, "#%s ", t)
	}

	return strings.TrimSpace(sb.String())
}

// TagsWithColorPound returns a prettified tags with #.
//
//	#tag1 #tag2 #tag3
func TagsWithColorPound(p *ansi.Palette, s string) string {
	tagsSplit := strings.Split(s, ",")
	sort.Strings(tagsSplit)

	var sb strings.Builder
	for _, t := range tagsSplit {
		if t == "" {
			continue
		}

		rc := p.Random(p.Italic)
		fmt.Fprintf(&sb, "%s%s ", rc.Sprint("#"), p.Italic.Sprint(t))
	}

	return sb.String()
}

// TagsColoredWithDelimiters returns prettified tags with custom left/right icons.
//
//	TagsColoredWithDelimiters(c, "tag1,tag2", "«", "»")  // «tag1» «tag2»
func TagsColoredWithDelimiters(p *ansi.Palette, tags []string, left, right string) string {
	sort.Strings(tags)
	var sb strings.Builder
	for _, t := range tags {
		if t == "" {
			continue
		}
		rc := p.Random()
		pill1, pill2 := rc.Sprint(left), rc.Sprint(right)
		tag := rc.Wrap(t, p.Inverse, p.Bold)
		sb.WriteString(pill1)
		sb.WriteString(tag)
		sb.WriteString(pill2)
		sb.WriteString(" ")
	}
	return sb.String()
}

// TagsWithPoundList returns a prettified tags list with #.
func TagsWithPoundList(s string) []string {
	return strings.FieldsFunc(TagsWithPound(s), func(r rune) bool { return r == ' ' })
}

// TagsWith returns a prettified tags.
//
//	tag1·tag2·tag3
func TagsWith(s, sep string) string {
	return strings.TrimRight(strings.ReplaceAll(s, ",", sep), sep)
}

// SpanCenter returns a line of exactly width characters with fill on both
// sides of the label.
//
//	-------- label --------
func SpanCenter(width int, label, char string) string {
	const spaces = 2
	if width < len(label)+spaces {
		return label
	}

	dashCount := width - len(label) - spaces
	left := dashCount / 2
	right := dashCount - left

	return strings.Repeat(char, left) + label + strings.Repeat(char, right)
}

// SpanSuffix returns a line of exactly width characters with fill after the
// label.
//
//	---------------- label
func SpanSuffix(width int, label, char string) string {
	const spaces = 2
	if width < len(label)+spaces {
		return label
	}
	dashCount := width - len(label) - spaces
	return label + strings.Repeat(char, dashCount)
}

// SpanPrefix returns a line of exactly width characters with fill before the
// label.
//
//	label ----------------
func SpanPrefix(width int, label, char string) string {
	const spaces = 2
	if width < len(label)+spaces {
		return label
	}

	dashCount := width - len(label) - spaces
	return strings.Repeat(char, dashCount) + label
}

// Span returns a line of exactly width characters with fill between two
// labels.
//
//	left ---------------- right
func Span(width int, left, right, char string) string {
	const spaces = 2
	dashCount := width - len(left) - len(right) - spaces
	return left + strings.Repeat(char, dashCount) + right
}

// CreateSimpleTable generates a simple ASCII table with basic borders.
func CreateSimpleTable(headers []string, rows [][]string, footer ...string) string {
	if len(headers) == 0 {
		return ""
	}

	// compute column widths ignoring ansi sequences
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = VisibleWidth(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				if w := VisibleWidth(cell); w > colWidths[i] {
					colWidths[i] = w
				}
			}
		}
	}

	var b strings.Builder

	writeBorder := func() {
		b.WriteByte('+')
		for _, width := range colWidths {
			b.WriteString(strings.Repeat("-", width+2))
			b.WriteByte('+')
		}
		b.WriteByte('\n')
	}

	writeRow := func(row []string) {
		b.WriteByte('|')
		for i, width := range colWidths {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}

			padding := width - VisibleWidth(cell)

			b.WriteByte(' ')
			b.WriteString(cell)
			b.WriteString(strings.Repeat(" ", padding))
			b.WriteString(" |")
		}
		b.WriteByte('\n')
	}

	writeBorder()
	writeRow(headers)
	writeBorder()

	for _, row := range rows {
		writeRow(row)
	}

	writeBorder()

	if len(footer) > 0 {
		totalWidth := 1
		for _, width := range colWidths {
			totalWidth += width + 3
		}
		totalWidth--

		for _, line := range footer {
			lineWidth := min(VisibleWidth(line), totalWidth)
			leftPad := (totalWidth - lineWidth) / 2

			b.WriteString(strings.Repeat(" ", leftPad))
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	return b.String()
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

func HTTPStatusCodeColor(statusCode int, p *ansi.Palette) ansi.Style {
	switch {
	case statusCode == 0:
		return p.BrightRed.With(p.Italic)

	// 2xx Success
	case statusCode >= 200 && statusCode < 300:
		return p.Green.With(p.Bold)

	// 3xx Redirection
	case statusCode >= 300 && statusCode < 400:
		return p.BrightBlue

	// Specific 4xx
	case statusCode == http.StatusUnauthorized:
		return p.Yellow.With(p.Bold)

	case statusCode == http.StatusForbidden:
		return p.BrightRed

	case statusCode == http.StatusNotFound:
		return p.Red.With(p.Bold)

	case statusCode == http.StatusGone:
		return p.Red.With(p.Bold)

	case statusCode == http.StatusTooManyRequests:
		return p.BrightYellow.With(p.Bold)

	// Other 4xx
	case statusCode >= 400 && statusCode < 500:
		return p.Yellow

	// Specific 5xx
	case statusCode == http.StatusGatewayTimeout:
		return p.BrightMagenta

	case statusCode == http.StatusBadGateway:
		return p.Magenta.With(p.Bold)

	case statusCode == http.StatusServiceUnavailable:
		return p.BrightMagenta.With(p.Bold)

	case statusCode == http.StatusInternalServerError:
		return p.BrightRed.With(p.Bold)

	// Other 5xx
	case statusCode >= 500 && statusCode < 600:
		return p.Magenta

	default:
		return p.Dim.With(p.Italic)
	}
}

func Pill(color, inverse ansi.Style, msg, sepLeft, sepRight string) string {
	var sb strings.Builder
	sb.WriteString(color.Sprint(sepLeft))
	sb.WriteString(color.Wrap(msg, inverse))
	sb.WriteString(color.Sprint(sepRight))
	return sb.String()
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
