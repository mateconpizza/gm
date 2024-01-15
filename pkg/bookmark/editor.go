// Copyright © 2023 haaag <git.haaag@gmail.com>
package bookmark

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gomarks/pkg/config"
)

var (
	ErrBufferUnchanged  = errors.New("buffer unchanged")
	ErrEditorNotFound   = errors.New("editor not found")
	ErrTooManyRecords   = errors.New("too many records")
	ErrBufferEndOfBlock = errors.New("end of the block not found")
)

type EditFn func(*[]Bookmark) error

type editorInfo struct {
	name string
	args []string
}

// ParseTempBookmark Parses the provided bookmark content into a temporary bookmark
// struct.
func ParseTempBookmark(content []string) *Bookmark {
	url := extractBlock(content, "## url", "## title")
	title := extractBlock(content, "## title", "## tags")
	tags := extractBlock(content, "## tags", "## description")
	desc := extractBlock(content, "## description", "## end")

	return &Bookmark{
		URL:   url,
		Title: title,
		Tags:  tags,
		Desc:  desc,
	}
}

func extractIDFromLine(line string) string {
	startIndex := strings.Index(line, "[")
	endIndex := strings.Index(line, "]")

	if startIndex == -1 || endIndex == -1 {
		return ""
	}

	return line[startIndex+1 : endIndex]
}

func strconvLineID(line string) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(line))

	if err != nil {
		if errors.Is(err, strconv.ErrSyntax) {
			return -1, fmt.Errorf("%w: '%s'", ErrInvalidRecordID, line)
		}
		return -1, fmt.Errorf("%w", err)
	}

	return id, nil
}

func extractIDsFromBuffer(data []byte) ([]int, error) {
	ids := make([]int, 0)
	lines := strings.Split(string(data), "\n")

	for _, l := range lines {
		s := extractIDFromLine(l)
		if s == "" {
			continue
		}

		id, err := strconvLineID(s)
		if err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func EditionSlice(bs *[]Bookmark) error {
	bsContent := Buffer(bs)
	data, err := Edit(bsContent)
	if err != nil {
		return err
	}

	if !isSameContentBytes(bsContent, data) {
		ids, err := extractIDsFromBuffer(data)
		if err != nil {
			return err
		}

		if len(ids) == 0 {
			return fmt.Errorf("%w", ErrBookmarkNotSelected)
		}

		FilterSliceByIDs(bs, ids...)
	}

	return nil
}

func Buffer(bs *[]Bookmark) []byte {
	var result strings.Builder

	for _, b := range *bs {
		id := fmt.Sprintf("[%d]", b.ID)
		result.WriteString(fmt.Sprintf("# %s %10s\n# tags: %s\n%s\n\n", id, b.Title, b.Tags, b.URL))
	}

	data := []byte(fmt.Sprintf(`## %s: v%s
## To keep a bookmark, remove the entire line starting with '#'

## Showing %d bookmarks.

%s`, config.App.Data.Title, config.App.Version, len(*bs), result.String()))

	return bytes.TrimSpace(data)
}

// Edit edits the provided bookmark using the specified editor.
func Edit(data []byte) ([]byte, error) {
	tf, err := createTempFile()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	tf, err = saveDataToTempFile(tf, data)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	defer func() {
		if err = tf.Close(); err != nil {
			log.Printf("Error closing temp file: %v", err)
		}
	}()

	defer func() {
		if err = cleanupTempFile(tf.Name()); err != nil {
			log.Printf("%v", err)
		}
	}()

	editor, err := getEditor()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if err = editFile(editor, tf); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	editedContent, err := readContentFile(tf)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if isSameContentBytes(data, editedContent) {
		return editedContent, fmt.Errorf("%w", ErrBufferUnchanged)
	}

	return editedContent, nil
}

// Checks if the buffer is unchanged
func isSameContentBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// saveDataToTempFile Writes the provided data to a temporary file and returns the file handle.
func saveDataToTempFile(f *os.File, data []byte) (*os.File, error) {
	const filePermission = 0o600

	err := os.WriteFile(f.Name(), data, filePermission)
	if err != nil {
		return nil, fmt.Errorf("error writing to temp file: %w", err)
	}

	return f, nil
}

func createTempFile() (*os.File, error) {
	tempDir := "/tmp/"
	prefix := fmt.Sprintf("%s-", config.App.Name)

	tempFile, err := os.CreateTemp(tempDir, prefix)
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	return tempFile, nil
}

// ValidateBookmarkBuffer validates the content of a bookmark file by ensuring that
// it contains both a URL and tags.
func ValidateBookmarkBuffer(content []string) error {
	url := extractBlock(content, "## url:", "## title:")
	tags := extractBlock(content, "## tags:", "## description:")

	if isEmptyLine(url) {
		return ErrBookmarkURLEmpty
	}

	if isEmptyLine(tags) {
		return ErrBookmarkTagsEmpty
	}

	return nil
}

// cleanupTempFile Removes the specified temporary file.
func cleanupTempFile(fileName string) error {
	err := os.Remove(fileName)
	if err != nil {
		return fmt.Errorf("could not cleanup temp file: %w", err)
	}

	return nil
}

// extractBlock Extracts a block of text from a string, delimited by the
// specified start and end markers.
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

// editFile Opens the specified file for editing using the provided editor.
func editFile(e *editorInfo, f *os.File) error {
	tempFileName := f.Name()

	// Construct the editor command with the temporary file path and editor arguments.
	cmd := exec.Command(e.name, append(e.args, tempFileName)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}

	return nil
}

// getEditor Retrieves the preferred editor to use for editing bookmarks.
func getEditor() (*editorInfo, error) {
	if appEditor, exists := getAppEditor(); exists {
		return appEditor, nil
	}

	editor := strings.Fields(os.Getenv("EDITOR"))
	if len(editor) > 0 {
		log.Printf("$EDITOR set to %s", editor)
		return &editorInfo{name: editor[0], args: editor[1:]}, nil
	}

	log.Printf("$EDITOR not set.")

	for _, e := range config.TextEditors {
		if binaryExists(e) {
			return &editorInfo{name: e}, nil
		}
	}

	return nil, ErrEditorNotFound
}

func getAppEditor() (*editorInfo, bool) {
	appEditor := strings.Fields(os.Getenv(config.App.Env.Editor))
	if len(appEditor) == 0 {
		return nil, false
	}

	if !binaryExists(appEditor[0]) {
		log.Printf("'%s' executable file not found in $PATH", appEditor[0])
		return nil, false
	}

	log.Printf("$%s set to %s", config.App.Env.Editor, appEditor)
	return &editorInfo{name: appEditor[0], args: appEditor[1:]}, true
}

func readContentFile(file *os.File) ([]byte, error) {
	tempFileName := file.Name()
	content, err := os.ReadFile(tempFileName)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return content, nil
}

func isEmptyLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

func EditAndRenderBookmarks(r *SQLiteRepository, bs *[]Bookmark, force bool) error {
	const tooManyRecords = 8
	if len(*bs) > tooManyRecords && !force {
		return fmt.Errorf("%w: %d. Max: %d", ErrTooManyRecords, len(*bs), tooManyRecords)
	}

	for i := range *bs {
		if err := (*bs)[i].Edit(r); err != nil {
			return fmt.Errorf("error editing bookmark: %w", err)
		}
	}

	return nil
}
