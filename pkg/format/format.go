package format

import (
	"fmt"
	"strconv"
	"strings"

	"gomarks/pkg/config"
)

var bulletPoint string = "\u2022"

func WithBullet(label, value string) string {
	padding := 15
	return fmt.Sprintf("    %s %-*s: %s\n", bulletPoint, padding, label, value)
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
	return fmt.Sprintf("%-4d\t%s %s\n", id, Text(bulletPoint).Purple().Bold(), title)
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

func CmdTitle(s string) {
	// FIX: Delete me
	program := fmt.Sprintf("%s:", config.App.Name)
	p := Text(program).White().Bold()
	t := Text(s).Blue()
	q := fmt.Sprintf("use %s for quit\n", Text("ctrl+c").Red().Bold())

	fmt.Println(p, t, q)
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

func ExtractMaxLen(l []string, target string) int {
	// FIX: delete this
	hasMaxLen := 2
	for _, s := range l {
		if strings.Contains(s, target) {
			parts := strings.Split(s, ":")
			if len(parts) == hasMaxLen {
				maxLen, err := strconv.Atoi(parts[1])
				if err == nil {
					return maxLen
				}
			}
		}
	}

	return 0
}

func Prompt(question, options string) string {
	q := Text(question).White().Bold()
	o := Text(options).Gray()
	return fmt.Sprintf("\n%s %s: ", q, o)
}
