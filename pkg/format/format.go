package format

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

var (
	BulletPoint      string = "\u2022"
	Space            string = "    "
	ErrInvalidOption error  = errors.New("invalid option")
)

func BulletLine(label, value string) string {
	padding := 15
	return fmt.Sprintf("%s%s %-*s: %s\n", Space, BulletPoint, padding, label, value)
}

func Title(title string, items []string) string {
	var s string

	t := fmt.Sprintf("> %s:\n", title)
	s += t

	for _, item := range items {
		s += item
	}

	return s
}

func ShortenString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}

	return s
}

func TitleLine(id int, title string) string {
	return fmt.Sprintf("%-4d%s%s %s\n", id, Space, BulletPoint, title)
}

func SplitAndAlignString(s string, lineLength int) string {
	var result string

	words := strings.Fields(s)
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 > lineLength {
			result += currentLine + "\n"
			currentLine = word
			currentLine = fmt.Sprintf("\t  %s", currentLine)
		} else {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		}
	}

	result += currentLine

	return result
}

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

func Prompt(question, options string) string {
	q := Text(question).White().Bold()
	o := Text(options).Gray()
	return fmt.Sprintf("\n%s %s ", q, o)
}

// convert: "tag1, tag2, tag3 tag"
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

func ToJSON(data interface{}) string {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}

	jsonString := string(jsonData)

	return jsonString
}
