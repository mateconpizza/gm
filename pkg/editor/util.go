package editor

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/haaag/gm/pkg/util/files"
)

// FIX: remove `tempExt`, it is being used for syntax highlight on edition.
const tempExt = "bookmark"

func createAndSave(d *[]byte) (*os.File, error) {
	tf, err := files.CreateTemp("bookmark", tempExt)
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	if err := saveDataToTempFile(tf, *d); err != nil {
		return nil, err
	}

	return tf, nil
}

// Content returns the content of a []byte as a slice of strings.
func Content(data *[]byte) []string {
	return strings.Split(string(*data), "\n")
}

// ExtractContentLine extracts URLs from the a slice of strings.
func ExtractContentLine(c *[]string) map[string]bool {
	m := make(map[string]bool)
	for _, l := range *c {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "#") && !strings.EqualFold(l, "") {
			m[l] = true
		}
	}

	return m
}

// saveDataToTempFile Writes the provided data to a temporary file and returns
// the file handle.
func saveDataToTempFile(f *os.File, data []byte) error {
	const filePermission = 0o600

	err := os.WriteFile(f.Name(), data, filePermission)
	if err != nil {
		return fmt.Errorf("error writing to temp file: %w", err)
	}

	return nil
}

// ExtractBlock extracts a block of text from a string, delimited by the
// specified start and end markers.
func ExtractBlock(content *[]string, startMarker, endMarker string) string {
	startIndex := -1
	endIndex := -1
	isInBlock := false

	var cleanedBlock []string

	for i, line := range *content {
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

// editFile executes a command to edit the specified file, logging errors if
// the command fails.
func editFile(fileName *os.File, command string, args []string) error {
	if command == "" {
		return ErrTextEditorNotFound
	}

	log.Printf("editing file: '%s'", fileName.Name())
	log.Printf("executing args: cmd='%s' args='%v'", command, args)
	cmd := exec.CommandContext(context.Background(), command, append(args, fileName.Name())...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}

	return nil
}

// readFileContent reads the content of the specified file into the given byte
// slice and returns any error encountered.
func readFileContent(fileName *os.File, data *[]byte) error {
	log.Printf("reading file: '%s'", fileName.Name())
	tempData, err := os.ReadFile(fileName.Name())
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// When reading the temporary file, the last line is always empty. This
	// causes the length of the []byte to differ. Let's trim the last line.
	*data = bytes.TrimSuffix(tempData, []byte{'\n'})

	return nil
}
