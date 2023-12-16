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

	"gomarks/pkg/app"
	"gomarks/pkg/format"
	"gomarks/pkg/util"
)

var (
	ErrBufferUnchanged = errors.New("buffer unchanged")
	ErrEditorNotFound  = errors.New("editor not found")
	ErrTooManyRecords  = errors.New("too many records")
)

type EditFn func(*Slice) error

type editorInfo struct {
	Name string
	Args []string
}

// ParseTempBookmark Parses the provided bookmark content into a temporary bookmark struct.
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

func EditBuffer(data []byte) ([]byte, error) {
	dataEdited, err := Edit(data)
	if err != nil {
		return dataEdited, err
	}
	return dataEdited, nil
}

func OldEditBuffer(data []byte) ([]byte, error) {
	dataEdited, err := Edit(data)
	if err != nil {
		if errors.Is(err, ErrBufferUnchanged) {
			fmt.Printf("%s\n", format.Text("unchanged").Yellow().Bold())
			return dataEdited, nil
		}
		return nil, fmt.Errorf("%w", err)
	}
	return dataEdited, nil
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

func EditionSlice(bs *Slice) error {
	bsContent := bs.Buffer()
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

// Edit Edits the provided bookmark using the specified editor.
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
		return b, fmt.Errorf("%w", err)
	}

	if err = editFile(editor, tf); err != nil {
		return b, fmt.Errorf("%w", err)
	}

	editedContent, err := readContentFile(tf)
	if err != nil {
		return b, fmt.Errorf("%w", err)
	}

	tempContent := strings.Split(string(editedContent), "\n")
	if err := validateContent(tempContent); err != nil {
		return b, err
	}

	if isSameContentBytes(data, editedContent) {
		return b, fmt.Errorf("%w", errs.ErrBookmarkUnchaged)
	}

	tb := parseTempBookmark(tempContent)
	tb.fetchTitleAndDescription()

	b.Update(tb.url, tb.title, tb.tags, tb.desc)
	return b, nil
}

// Checks if the buffer is unchanged
func isSameContentBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}

/**
 * Writes the provided data to a temporary file and returns the file handle.
 *
 * @param file The temporary file to write to.
 * @param data The data to write to the file.
 *
 * @return The file handle of the temporary file, or an error if the data could not be written to the file.
 */
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
	prefix := fmt.Sprintf("%s-", app.Config.Name)

	tempFile, err := os.CreateTemp(tempDir, prefix)
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	return tempFile, nil
}

/**
 * Validates the content of a bookmark file by ensuring that it contains both a URL and tags.
 *
 * @param content The content of the bookmark file.
 *
 * @return An error if the content is invalid.
 */
func validateContent(content []string) error {
	url := extractBlock(content, "## url:", "## title:")
	tags := extractBlock(content, "## tags:", "## description:")

	if isEmptyLine(url) {
		return errs.ErrURLEmpty
	}

	if isEmptyLine(tags) {
		return errs.ErrTagsEmpty
	}

	return nil
}

/**
 * Removes the specified temporary file.
 *
 * @param fileName The name of the temporary file to remove.
 *
 * @return An error if the temporary file could not be removed.
 */
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

/**
 * Opens the specified file for editing using the provided editor.
 *
 * @param editor The editor information struct containing the editor name and arguments.
 * @param file The file to open for editing.
 *
 * @return An error if an error occurs while opening or editing the file.
 */
func editFile(e *editorInfo, f *os.File) error {
	tempFileName := f.Name()

	// Construct the editor command with the temporary file path and editor arguments.
	cmd := exec.Command(e.Name, append(e.Args, tempFileName)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}

	return nil
}

/**
 * Retrieves the preferred editor to use for editing bookmarks.
 *
 * @return A pointer to an editorInfo struct containing the editor name and
 * arguments, or an error if no editor could be found.
 */
func getEditor() (*editorInfo, error) {
	if appEditor, exists := getAppEditor(); exists {
		return appEditor, nil
	}

	editor := strings.Fields(os.Getenv("EDITOR"))
	if len(editor) > 0 {
		log.Printf("$EDITOR set to %s", editor)
		return &editorInfo{Name: editor[0], Args: editor[1:]}, nil
	}

	log.Printf("$EDITOR not set.")

	for _, e := range app.Editors {
		if util.BinaryExists(e) {
			return &editorInfo{Name: e}, nil
		}
	}

	return nil, errs.ErrEditorNotFound
}

func getAppEditor() (*editorInfo, bool) {
	appEditor := strings.Fields(os.Getenv(app.Env.Editor))
	if len(appEditor) == 0 {
		return nil, false
	}

	if !util.BinaryExists(appEditor[0]) {
		log.Printf("'%s' executable file not found in $PATH", appEditor[0])
		return nil, false
	}

	log.Printf("$%s set to %s", app.Env.Editor, appEditor)
	return &editorInfo{Name: appEditor[0], Args: appEditor[1:]}, true
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
