package editor

import (
	"fmt"
	"strings"
)

// Append inserts a header string at the beginning of a byte
// buffer
func Append(s string, buf *[]byte) {
	*buf = append([]byte(s), *buf...)
}

// AppendVersion inserts a header string at the beginning of a byte
// buffer
func AppendVersion(name, version string, buf *[]byte) {
	Append(fmt.Sprintf("## %s: v%s\n", name, version), buf)
}

// isEmptyLine checks if a line is empty
func isEmptyLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// Validate checks if the URL and Tags are in the content
func Validate(content *[]string) error {
	if err := validateURLBuffer(content); err != nil {
		return err
	}
	return validateTagsBuffer(content)
}

// validateURLBuffer validates url in the buffer
func validateURLBuffer(content *[]string) error {
	url := ExtractBlock(content, "# URL:", "# Title:")
	if isEmptyLine(url) {
		return fmt.Errorf("%w: URL", ErrLineNotFound)
	}
	return nil
}

// validateTagsBuffer validates tags in the buffer
func validateTagsBuffer(content *[]string) error {
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
