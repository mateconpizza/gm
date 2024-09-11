package editor

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/haaag/gm/internal/util/files"
)

// AppendBuffer inserts a header string at the beginning of a byte buffer.
func AppendBuffer(s string, buf *[]byte) {
	*buf = append([]byte(s), *buf...)
}

// AppendVersion inserts a header string at the beginning of a byte buffer.
func AppendVersion(name, version string, buf *[]byte) {
	AppendBuffer(fmt.Sprintf("# %s: v%s\n", name, version), buf)
}

// IsEmptyLine checks if a line is empty.
func IsEmptyLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// Edit edits the contents of a byte slice by creating a temporary file,
// editing it with an external editor, and then reading the modified contents
// back into the byte slice.
func Edit(buf *[]byte) error {
	tf, err := createAndSave(buf)
	if err != nil {
		return err
	}
	defer files.Cleanup(tf)

	if err := editFile(tf, Editor.cmd, Editor.args); err != nil {
		return err
	}

	return readFileContent(tf, buf)
}

// CopyBuffer copies the contents of a byte slice into a new byte slice.
func CopyBuffer(buf *[]byte) []byte {
	return append([]byte(nil), *buf...)
}

// IsSameContentBytes Checks if the buffer is unchanged.
func IsSameContentBytes(a, b *[]byte) bool {
	return bytes.Equal(*a, *b)
}
