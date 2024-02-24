package format

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

var (
	BulletPoint      = "\u2022"
	Space            = "\u0020"
	ErrInvalidOption = errors.New("invalid option")
)

// BulletLine returns a formatted string with a label and a value
func BulletLine(label, value string) string {
	padding := 15
	return fmt.Sprintf("%s%s %-*s: %s\n", Space, BulletPoint, padding, label, value)
}

// HeaderWithSection returns a formatted string with a title and a list of items
func HeaderWithSection(title string, items []string) string {
	var result strings.Builder

	t := fmt.Sprintf("> %s:\n", title)
	result.WriteString(t)

	for _, item := range items {
		result.WriteString(item)
	}

	return result.String()
}

// ShortenString shortens a string to a maximum length
func ShortenString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}

	return s
}

// HeaderLine returns a formatted string with a title
func HeaderLine(id int, titles ...string) string {
	padding := 6
	return fmt.Sprintf("%-*d%s %s\n", padding, id, BulletPoint, strings.Join(titles, " "))
}

// SplitAndAlignString splits a string into multiple lines and aligns the words
func SplitAndAlignString(s string, lineLength, indentation int) string {
	var sep = strings.Repeat(Space, indentation)

	var result strings.Builder
	var currentLine strings.Builder

	for _, word := range strings.Fields(s) {
		if currentLine.Len()+len(word)+1 > lineLength {
			result.WriteString(currentLine.String())
			result.WriteString("\n")
			currentLine.Reset()
			currentLine.WriteString(sep)
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

// ParseUniqueStrings returns a slice of unique strings
func ParseUniqueStrings(input []string, sep string) []string {
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

// Prompt returns a formatted string with a question and options
func Prompt(question, options string) string {
	q := Text(question).White().Bold()
	o := Text(options).Gray()
	return fmt.Sprintf("\n%s %s ", q, o)
}

// ParseTags normalizes a string of tags by separating them by commas and ensuring that the final string ends with a comma.
//
// from: "tag1, tag2, tag3 tag"
// to: "tag1,tag2,tag3,tag,"
func ParseTags(tags string) string {
	tags = strings.Join(strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	}), ",")

	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

// ToJSON converts an interface to JSON
func ToJSON(data interface{}) []byte {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("error converting to JSON: %s", err)
	}

	return jsonData
}

// filterEmptyStrings removes empty strings from a slice
func filterEmptyStrings(s []string) []string {
	var result []string
	for _, str := range s {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}
