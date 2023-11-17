package format

import (
	"fmt"
	"strings"

	"gomarks/pkg/color"
	"gomarks/pkg/config"
)

func ShortenString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}

	return s
}

func FormatLine(prefix, v, c string) string {
	if c == "" {
		return fmt.Sprintf("%s%s\n", prefix, v)
	}

	return fmt.Sprintf("%s%s%s%s\n", c, prefix, v, color.Reset)
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
	program := fmt.Sprintf("%s:", config.App.Name)
	p := color.ColorizeBold(program, color.White)
	t := color.Colorize(s, color.Blue)
	quit := color.ColorizeBold("ctrl+c", color.Red)
	q := fmt.Sprintf("use %s for quit\n", quit)

	fmt.Println(p, t, q)
}

func FormatTitleLine(n int, title, c string) string {
	if title == "" {
		title = "Untitled"
	}

	if c == "" {
		return fmt.Sprintf("%-4d\t%s %s\n", n, config.BulletPoint, title)
	}

	return fmt.Sprintf(
		"%s%-4d\t%s%s %s%s\n",
		color.Bold,
		n,
		config.BulletPoint,
		c,
		title,
		color.Reset,
	)
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
