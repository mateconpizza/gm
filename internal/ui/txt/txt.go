package txt

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

const (
	UnicodeBulletPoint      = "\u2022" // •
	UnicodeLightDiagCross   = "\u2571" // ╱
	UnicodeMiddleDot        = "\u00b7" // ·
	UnicodePathBigSegment   = "\u25B6" // ▶
	UnicodePathSmallSegment = "\u25B8" // ▸
	UnicodeRightDoubleAngle = "\u00BB" // »
	UnicodeSingleAngleMark  = "\u203A" // ›
)

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
	separator := strings.Repeat(" ", indentation)
	var result strings.Builder
	var currentLine strings.Builder

	for _, word := range strings.Fields(s) {
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
	var lines []string
	var currentLine strings.Builder

	// Remember if the original string ended with a newline.
	endsWithNewline := strings.HasSuffix(s, "\n")

	for _, word := range strings.Fields(s) {
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
func URLBreadCrumbs(s string, c color.ColorFn) string {
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

// DiffColor colorizes the diff output.
func DiffColor(s string) string {
	var r []string
	for l := range strings.SplitSeq(s, "\n") {
		switch {
		case strings.HasPrefix(l, "+"):
			r = append(r, "  "+color.BrightGreen(l).String())
		case strings.HasPrefix(l, "-"):
			r = append(r, "  "+color.BrightRed(l).String())
		default:
			r = append(r, "  "+color.BrightGray(l).Italic().String())
		}
	}

	return strings.Join(r, "\n")
}

// RelativeTime takes a timestamp string in the format "20060102-150405"
// and returns a relative description.
//
//	"today", "yesterday" or "X days ago"
func RelativeTime(ts string) string {
	const layout = "20060102-150405"

	t, err := time.Parse(layout, ts)
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

// SuccessMesg returns a prettified success message.
func SuccessMesg(s string) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	success := color.BrightGreen("Successfully ").Italic().String()
	message := success + color.Text(s).Italic().String()
	return f.Reset().Success(message).String()
}

// WarningMesg returns a prettified warning message.
func WarningMesg(s string) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	warning := color.BrightYellow("Warning ").Italic().String()
	message := warning + color.Text(s).Italic().String()
	return f.Reset().Warning(message).String()
}

// ErrorMesg returns a prettified error message.
func ErrorMesg(s string) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	err := color.BrightRed("Error ").Italic().String()
	message := err + color.Text(s).Italic().String()
	return f.Reset().Error(message).String()
}

// InfoMesg returns a prettified info message.
func InfoMesg(s string) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	info := color.BrightBlue("Info ").Italic().String()
	message := info + color.Text(s).Italic().String()
	return f.Reset().Info(message).String()
}

// ExtractBlock extracts a block of text from a string, delimited by the
// specified start and end markers.
func ExtractBlock(content []string, startMarker, endMarker string) string {
	startIndex := -1
	endIndex := -1
	isInBlock := false

	var cleanedBlock []string

	for i, line := range content {
		if strings.HasPrefix(line, startMarker) {
			startIndex = i
			isInBlock = true

			continue
		}

		if strings.HasPrefix(line, endMarker) && isInBlock {
			endIndex = i

			break // Found end marker line
		}

		if isInBlock {
			cleanedBlock = append(cleanedBlock, line)
		}
	}

	if startIndex == -1 || endIndex == -1 {
		return ""
	}

	return strings.Join(cleanedBlock, "\n")
}
