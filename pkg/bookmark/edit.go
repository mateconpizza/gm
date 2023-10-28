package bookmark

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"gomarks/pkg/scrape"
)

func Edit(b *Bookmark) (*Bookmark, error) {
	data := editTempContent(b)
	tempFile := saveDataToTemporaryFile(data)

	err := editFile(tempFile)
	if err != nil {
		return b, err
	}

	editedContent := readFile(tempFile)
	content := parseEditedContent(editedContent)

	if isSameContentBytes(data, editedContent) {
		return b, fmt.Errorf("no changes made. editing cancelled")
	}

	b.setURL(content[0])
	b.setTitle(content[1])
	b.setTags(content[2])
	b.setDesc(content[3])

	if b.Title.String == "" || b.Desc.String == "" {
		result, err := scrape.TitleAndDescription(b.URL)
		if err != nil {
			return b, err
		}
		b.setTitle(result.Title)
		b.setDesc(result.Description)
	}

	if !b.IsValid() {
		return b, fmt.Errorf("invalid bookmark: %s", b)
	}

	cleanupTemporaryFile(tempFile)
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

func editFile(file string) error {
	editor := getEditor()
	cmd := exec.Command(editor, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readFile(file string) []byte {
	content, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	return content
}

func editTempContent(b *Bookmark) []byte {
	data := []byte(fmt.Sprintf(`## Editing %s
## lines starting with # will be ignored.
## url:
%s
## title: (leave empty line for web fetch)
%s
## tags: (comma separated)
%s
## description: (leave empty line for web fetch)
%s
## End
`, b.URL, b.URL, b.Title.String, b.Tags, b.Desc.String))
	return bytes.TrimRight(data, " ")
}

func getEditor() string {
	Editor := os.Getenv("EDITOR")
	if Editor == "" {
		log.Printf("Var $EDITOR not set.")
	}
	if binaryExists("vim") {
		Editor = "vim"
		return Editor
	}
	if binaryExists("nano") {
		Editor = "nano"
		return Editor
	}
	if binaryExists("nvim") {
		Editor = "nvim"
		return Editor
	}
	return Editor
}

func binaryExists(binaryName string) bool {
	cmd := exec.Command("which", binaryName)
	err := cmd.Run()
	return err == nil
}

func isValidContent(content []string) bool {
	const linesInContent = 11
	return len(content) == linesInContent
}

func parseEditedContent(content []byte) []string {
	// BUG: I dont know Rick. Make some tests
	resultLines := []string{}
	str := string(content)
	lines := strings.Split(strings.TrimSpace(str), "\n")
	if !isValidContent(lines) {
		errMsg := "Invalid content"
		fmt.Println(errMsg)
		log.Fatal(errMsg)
	}
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			resultLines = append(resultLines, line)
		}
	}
	return resultLines
}

func isSameContentBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}

func cleanupTemporaryFile(file string) {
	err := os.Remove(file)
	if err != nil {
		panic(err)
	}
}
