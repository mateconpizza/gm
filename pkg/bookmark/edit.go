package bookmark

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"gomarks/pkg/errs"
	"gomarks/pkg/scrape"
	"gomarks/pkg/util"
)

type tempBookmark struct {
	url   string
	title string
	tags  string
	desc  string
}

func getTempBookmark(content []string) *tempBookmark {
	url := extractBlock(content, "## url", "## title")
	title := extractBlock(content, "## title", "## tags")
	tags := extractBlock(content, "## tags", "## description")
	desc := extractBlock(content, "## description", "## end")

	return &tempBookmark{
		url:   url,
		title: title,
		tags:  tags,
		desc:  desc,
	}
}

// FIX: Necessary?
func getTitleAndDescription(t *tempBookmark) {
	if t.title == "" {
		title, err := scrape.GetTitle(t.url)
		if err != nil {
			log.Printf("Error on %s: %s\n", t.url, err)
		}
		t.title = title
	}

	if t.desc == "" {
		description, err := scrape.GetDescription(t.url)
		if err != nil {
			log.Printf("Error on %s: %s\n", t.url, err)
		}
		t.desc = description
	}
}

func Edit(b *Bookmark) (*Bookmark, error) {
	data := editTempContent(b)
	tempFile := saveDataToTemporaryFile(data)

	err := util.EditFile(tempFile)
	if err != nil {
		return b, fmt.Errorf("%w: editing bookmark", err)
	}

	editedContent := util.ReadFile(tempFile)
	tempContent := strings.Split(string(editedContent), "\n")

	if err := validateContent(tempContent); err != nil {
		return b, err
	}

	cleanupTemporaryFile(tempFile)

	if util.IsSameContentBytes(data, editedContent) {
		return b, errs.ErrBookmarkUnchaged
	}

	tempBookmark := getTempBookmark(tempContent)
	getTitleAndDescription(tempBookmark)

	b.URL = tempBookmark.url
	b.Title.String = tempBookmark.title
	b.Tags = tempBookmark.tags
	b.Desc.String = tempBookmark.desc

	return b, nil
}

// FIX: replace with tempFile.
func saveDataToTemporaryFile(data []byte) string {
	const filePermission = 0o600

	tempFile := "/tmp/gomarks-data-temp.md"
	err := os.WriteFile(tempFile, data, filePermission)
	if err != nil {
		panic(err)
	}

	return tempFile
}

func editTempContent(b *Bookmark) []byte {
	// FIX: replace with b.Buffer()
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
	url := extractBlock(content, "## url:", "## title:")
	tags := extractBlock(content, "## tags:", "## description:")

	if util.IsEmptyLine(url) {
		return errs.ErrURLEmpty
	} else if util.IsEmptyLine(tags) {
		return errs.ErrTagsEmpty
	}

	return nil
}

func cleanupTemporaryFile(file string) {
	err := os.Remove(file)
	if err != nil {
		panic(err)
	}
}

func extractBlock(content []string, startMarker, endMarker string) string {
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
