// Package format provides utilities for formatting strings
package format

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
)

var (
	BulletPoint      = "\u2022"
	ErrInvalidOption = errors.New("invalid option")
)

// ParseUniqueStrings returns a slice of unique strings
func parseUniqueStrings(input []string, sep string) []string {
	uniqueTags := make([]string, 0)
	uniqueMap := make(map[string]struct{})

	for _, tags := range input {
		tagList := strings.Split(tags, sep)
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				uniqueMap[tag] = struct{}{}
			}
		}
	}

	for tag := range uniqueMap {
		uniqueTags = append(uniqueTags, tag)
	}

	return uniqueTags
}

// urlPath returns a prettified URL
func urlPath(bURL string) string {
	u, err := url.Parse(bURL)
	if err != nil {
		return ""
	}

	if u.Host == "" || u.Path == "" {
		return Color(bURL).Bold().String()
	}

	host := Color(u.Host).Bold().String()
	pathSegments := strings.FieldsFunc(strings.TrimLeft(u.Path, "/"), func(r rune) bool { return r == '/' })

	if len(pathSegments) == 0 {
		return host
	}

	pathSeg := Color(BulletPoint, strings.Join(pathSegments, fmt.Sprintf(" %s ", BulletPoint))).Gray()
	return fmt.Sprintf("%s %s", host, pathSeg)
}

// BulletLine returns a formatted string with a label and a value
func BulletLine(label, value string) string {
	padding := 15
	return fmt.Sprintf("+ %-*s %s\n", padding, label, value)
}

// HeaderWithSection returns a formatted string with a title and a list of items
func HeaderWithSection(title string, items []string) string {
	var result strings.Builder

	t := fmt.Sprintf("%s:\n", title)
	result.WriteString(t)

	for _, item := range items {
		result.WriteString(item)
	}

	return result.String()
}

// HeaderLine returns a formatted string with a title
func HeaderLine(id int, titles ...string) string {
	padding := 6
	return fmt.Sprintf("%-*d%s %s\n", padding, id, BulletPoint, strings.Join(titles, " "))
}

// Header returns a formatted string with a title
func Header(s string) string {
	return fmt.Sprintf("%s:\n\n", s)
}

// ShortenString shortens a string to a maximum length
func ShortenString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}

	return s
}

// SplitAndAlignString splits a string into multiple lines and aligns the words
func SplitAndAlignString(s string, lineLength, indentation int) string {
	var separator = strings.Repeat(" ", indentation)
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

// ParseTags normalizes a string of tags by separating them by commas and ensuring that the final string ends with a comma.
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

// ToJSON converts an interface to JSON
func ToJSON(data any) []byte {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("error converting to JSON: %s", err)
	}

	return jsonData
}

// prettifyURL returns a prettified URL
func prettifyURL(bURL string) string {
	u, err := url.Parse(bURL)
	if err != nil {
		return ""
	}

	if u.Host == "" || u.Path == "" {
		return Color(bURL).Bold().String()
	}

	host := Color(u.Host).Bold().String()
	pathSegments := strings.FieldsFunc(strings.TrimLeft(u.Path, "/"), func(r rune) bool { return r == '/' })

	if len(pathSegments) == 0 {
		return host
	}

	pathSeg := Color(BulletPoint, strings.Join(pathSegments, fmt.Sprintf(" %s ", BulletPoint))).Gray()
	return fmt.Sprintf("%s %s", host, pathSeg)
}

// prettifyTags returns a prettified tags
func prettifyTags(s string) string {
	t := strings.ReplaceAll(s, ",", BulletPoint)
	return strings.TrimRight(t, BulletPoint)
}
