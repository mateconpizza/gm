package editor

import (
	"fmt"
	"strings"
)

// AppendBuffer inserts a header string at the beginning of a byte
// buffer
func AppendBuffer(s string, buf *[]byte) {
	*buf = append([]byte(s), *buf...)
}

// AppendVersionBuffer inserts a header string at the beginning of a byte
// buffer
func AppendVersionBuffer(name, version string, buf *[]byte) {
	AppendBuffer(fmt.Sprintf("## %s: v%s\n", name, version), buf)
}

// isEmptyLine checks if a line is empty
func isEmptyLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// Validate checks if the URL and Tags are in the content
func Validate(content *[]string) error {
	url := ExtractBlock(content, "# URL:", "# Title:")
	if isEmptyLine(url) {
		return fmt.Errorf("%w: URL", ErrLineNotFound)
	}
	tags := ExtractBlock(content, "# Tags:", "# Description:")
	if isEmptyLine(tags) {
		return fmt.Errorf("%w: Tags", ErrLineNotFound)
	}
	return nil
}

// ValidateURLBuffer validates url in the buffer
func ValidateURLBuffer(content *[]string) error {
	url := ExtractBlock(content, "# URL:", "# Title:")
	if isEmptyLine(url) {
		return fmt.Errorf("%w: URL", ErrLineNotFound)
	}
	return nil
}

// ValidateTagsBuffer validates tags in the buffer
func ValidateTagsBuffer(content *[]string) error {
	tags := ExtractBlock(content, "# Tags:", "# Description:")
	if isEmptyLine(tags) {
		return fmt.Errorf("%w: Tags", ErrLineNotFound)
	}
	return nil
}

// Edit edits the contents of a byte slice by creating a temporary file,
// editing it with an external editor, and then reading the modified contents
// back into the byte slice.
func Edit(buf *[]byte) error {
	tf, err := createAndSave(buf)
	if err != nil {
		return err
	}
	defer cleanup(tf)

	if err := editFile(tf, Editor.cmd, Editor.args); err != nil {
		return err
	}

	return readFileContent(tf, buf)
}
