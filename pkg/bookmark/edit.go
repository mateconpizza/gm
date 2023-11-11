package bookmark

import (
	"fmt"
	"log"
	"os"
	"os/exec"
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

func (t *tempBookmark) fetchTitleAndDescription() {
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

func parseTempBookmark(content []string) *tempBookmark {
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

func Edit(b *Bookmark) (*Bookmark, error) {
	tf := createTempFile()

	data := b.Buffer()
	tf = saveDataToTempFile(tf, data)

	defer func() {
		if err := tf.Close(); err != nil {
			panic(err)
		}
	}()

	editor, err := getEditor()
	if err != nil {
		return b, fmt.Errorf("bookmark edition: %w", err)
	}

	err = editFile(editor, tf)
	if err != nil {
		return b, fmt.Errorf("bookmark edition: %w", err)
	}

	editedContent, err := util.ReadContentFile(tf)
	if err != nil {
		return b, fmt.Errorf("bookmark edition: %w", err)
	}
	tempContent := strings.Split(string(editedContent), "\n")

	if err := validateContent(tempContent); err != nil {
		return b, err
	}

	cleanupTempFile(tf.Name())

	if util.IsSameContentBytes(data, editedContent) {
		return b, fmt.Errorf("bookmark edition: %w", errs.ErrBookmarkUnchaged)
	}

	tb := parseTempBookmark(tempContent)
	tb.fetchTitleAndDescription()

	b.Update(tb.url, tb.title, tb.tags, tb.desc)
	return b, nil
}

func saveDataToTempFile(file *os.File, data []byte) *os.File {
	const filePermission = 0o600

	err := os.WriteFile(file.Name(), data, filePermission)
	if err != nil {
		panic(err)
	}

	return file
}

func createTempFile() *os.File {
	tempDir := "/tmp/"
	prefix := "gomarks-"

	tempFile, err := os.CreateTemp(tempDir, prefix)
	if err != nil {
		panic(err)
	}

	return tempFile
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

func cleanupTempFile(fileName string) {
	err := os.Remove(fileName)
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

func editFile(editor string, file *os.File) error {
	tempFileName := file.Name()

	cmd := exec.Command(editor, tempFileName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}

	return nil
}

func getEditor() (string, error) {
	gomarksEditor := os.Getenv("GOMARKS_EDITOR")
	if gomarksEditor != "" {
		log.Printf("$GOMARKS_EDITOR set to %s", gomarksEditor)
		return gomarksEditor, nil
	}

	editor := os.Getenv("EDITOR")
	if editor != "" {
		log.Printf("$EDITOR set to %s", editor)
		return editor, nil
	}

	log.Printf("$EDITOR not set.")

	editors := []string{"vim", "nvim", "nano", "emacs"}
	for _, e := range editors {
		if util.BinaryExists(e) {
			return e, nil
		}
	}

	return "", errs.ErrEditorNotFound
}
