// Package format provides utilities for formatting strings
package format

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/haaag/gm/pkg/format/color"
)

const (
	_bulletPoint    = "\u2022" // •
	_midBulletPoint = "\u00b7" // ·
	_pathSegSep     = "\u25B8" // ▸
)

var ErrInvalidOption = errors.New("invalid option")

// URLPath returns a prettified URL.
func URLPath(bURL string) string {
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
		_pathSegSep,
		strings.Join(pathSegments, fmt.Sprintf(" %s ", _pathSegSep)),
	)

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// BulletLine returns a formatted string with a label and a value.
func BulletLine(label, value string) string {
	padding := 15
	return fmt.Sprintf("%-*s %s", padding, label, value)
}

// HeaderWithSection returns a formatted string with a title and a list of items.
func HeaderWithSection(title string, items []string) string {
	var r strings.Builder
	t := title + "\n"

	r.WriteString(t)
	for _, item := range items {
		r.WriteString(item)
	}

	return r.String()
}

// headerIDLine returns a formatted string with a title.
func headerIDLine(id int, titles ...string) string {
	padding := 6
	return fmt.Sprintf("%-*d%s %s\n", padding, id, _bulletPoint, strings.Join(titles, " "))
}

// Header returns a formatted string with a title.
func Header(s string) string {
	return s + "\n\n"
}

// ShortenString shortens a string to a maximum length.
func ShortenString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}

	return s
}

// SplitAndAlignLines splits a string into multiple lines and aligns the
// words.
func SplitAndAlignLines(s string, lineLength, indentation int) string {
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

// ParseTags normalizes a string of tags by separating them by commas and
// ensuring that the final string ends with a comma.
//
// from: "tag1, tag2, tag3 tag"
// to: "tag1,tag2,tag3,tag,"
func ParseTags(tags string) string {
	if tags == "" {
		return "notag"
	}
	tags = strings.Join(strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	}), ",")

	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

// ToJSON converts an interface to JSON.
func ToJSON(data any) []byte {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("error converting to JSON: %s", err)
	}

	return jsonData
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
		_pathSegSep,
		strings.Join(pathSegments, fmt.Sprintf(" %s ", _pathSegSep)),
	)

	return fmt.Sprintf("%s %s", host, pathSeg)
}

// PrettifyTags returns a prettified tags.
func PrettifyTags(s string) string {
	t := strings.ReplaceAll(s, ",", _midBulletPoint)
	return strings.TrimRight(t, _midBulletPoint)
}

func Printer(s ...string) {
}

// SplitIntoLines splits string into chunks of a given length.
func SplitIntoLines(s string, strLen int) []string {
	var lines []string
	var currentLine strings.Builder

	for _, word := range strings.Fields(s) {
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

	return lines
}

// AlignLines adds indentation to each line.
func AlignLines(lines []string, indentation int) []string {
	separator := strings.Repeat(" ", indentation)
	alignedLines := make([]string, 0, len(lines))

	for _, line := range lines {
		alignedLines = append(alignedLines, separator+line)
	}

	return alignedLines
}
