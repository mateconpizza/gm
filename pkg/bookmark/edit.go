package bookmark

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"gomarks/pkg/scrape"
	"gomarks/pkg/util"
)

type TempBookmark struct {
	url   string
	title string
	tags  string
	desc  string
}

func getTempBookmark(content []string) *TempBookmark {
	url := ExtractBlock(content, "## url", "## title")
	title := ExtractBlock(content, "## title", "## tags")
	tags := ExtractBlock(content, "## tags", "## description")
	desc := ExtractBlock(content, "## description", "## end")
	return &TempBookmark{
		url:   url,
		title: title,
		tags:  tags,
		desc:  desc,
	}
}

func getTitleAndDescription(t *TempBookmark) {
	var scrapeResult *scrape.ScrapeResult
	var err error
	if t.title == "" || t.desc == "" {
		scrapeResult, err = scrape.TitleAndDescription(t.url)
		if err != nil {
			log.Printf("Error on %s: %s\n", t.url, err)
		}
	}
	if t.title == "" {
		t.title = scrapeResult.Title
	}
	if t.desc == "" {
		t.desc = scrapeResult.Description
	}
}

func Edit(b *Bookmark) (*Bookmark, error) {
	data := editTempContent(b)
	tempFile := saveDataToTemporaryFile(data)

	err := util.EditFile(tempFile)
	if err != nil {
		return b, err
	}

	editedContent := util.ReadFile(tempFile)
	tempContent := strings.Split(string(editedContent), "\n")

	if err = validateContent(tempContent); err != nil {
		return b, err
	}

	cleanupTemporaryFile(tempFile)

	if util.IsSameContentBytes(data, editedContent) {
		return b, nil
	}

	tempBookmark := getTempBookmark(tempContent)
	getTitleAndDescription(tempBookmark)

	b.URL = tempBookmark.url
	b.Title.String = tempBookmark.title
	b.Tags = tempBookmark.tags
	b.Desc.String = tempBookmark.desc
	return b, nil
}

func saveDataToTemporaryFile(data []byte) string {
	tempFile := "/tmp/gomarks-data-temp.md"
	err := os.WriteFile(tempFile, data, 0o666)
	if err != nil {
		panic(err)
	}
	return tempFile
}

func editTempContent(b *Bookmark) []byte {
	data := []byte(fmt.Sprintf(`## Editing [%d] %s
## lines starting with # will be ignored.
## url:
%s
## title: (leave empty line for web fetch)
%s
## tags: (comma separated)
%s
## description: (leave empty line for web fetch)
%s
## end
`, b.ID, b.URL, b.URL, b.Title.String, b.Tags, b.Desc.String))
	return bytes.TrimRight(data, " ")
}

func validateContent(content []string) error {
	url := ExtractBlock(content, "## url:", "## title:")
	tags := ExtractBlock(content, "## tags:", "## description:")

	if util.IsEmptyLine(url) {
		return fmt.Errorf("url is empty")
	} else if util.IsEmptyLine(tags) {
		return fmt.Errorf("tags is empty")
	}
	return nil
}

func cleanupTemporaryFile(file string) {
	err := os.Remove(file)
	if err != nil {
		panic(err)
	}
}

func ExtractBlock(content []string, startMarker, endMarker string) string {
	startIndex := -1
	endIndex := -1
	isInBlock := false
	var cleanedBlock []string

	for i, line := range content {
		if strings.HasPrefix(line, startMarker) {
			startIndex = i
			isInBlock = true
			continue // Skip start marker line
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
