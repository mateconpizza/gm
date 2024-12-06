package files

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/sys"
)

var (
	ErrCommandNotFound    = errors.New("command not found")
	ErrTextEditorNotFound = errors.New("text editor not found")
)

// Fallback text editors if $EDITOR || $GOMARKS_EDITOR var is not set.
var textEditors = []string{"vim", "nvim", "nano", "emacs"}

type TextEditor struct {
	name string
	cmd  string
	args []string
}

// Editor retrieves the preferred editor to use for editing
//
// If env variable `GOMARKS_EDITOR` is not set, uses the `EDITOR`.
// If env variable `EDITOR` is not set, uses the first available
// `TextEditors`
//
// # fallbackEditors: `"vim", "nvim", "nano", "emacs"`.
func Editor(s string) (*TextEditor, error) {
	envs := []string{s, "EDITOR"}

	// find $EDITOR and $GOMARKS_EDITOR
	for _, e := range envs {
		if editor, found := getEditorFromEnv(e); found {
			if editor.cmd == "" {
				return nil, fmt.Errorf("%w: '%s'", ErrTextEditorNotFound, editor.name)
			}

			return editor, nil
		}
	}

	log.Printf(
		"$EDITOR and $GOMARKS_EDITOR not set, checking fallback text editor: %s",
		textEditors,
	)

	// find fallback
	if editor, found := getFallbackEditor(textEditors); found {
		return editor, nil
	}

	return nil, ErrTextEditorNotFound
}

// getEditorFromEnv finds an editor in the environment.
func getEditorFromEnv(e string) (*TextEditor, bool) {
	s := strings.Fields(sys.Env(e, ""))
	if len(s) != 0 {
		editor := newTextEditor(sys.BinPath(s[0]), s[0], s[1:])
		log.Printf("$EDITOR set: '%v'", editor)
		return editor, true
	}

	return nil, false
}

// getFallbackEditor finds a fallback editor.
func getFallbackEditor(editors []string) (*TextEditor, bool) {
	// FIX: use `exec.LookPath`
	// This will replace `sys.BinExists` and `sys.BinPath`
	for _, e := range editors {
		if sys.BinExists(e) {
			editor := newTextEditor(sys.BinPath(e), e, []string{})
			log.Printf("found fallback text editor: '%v'", editor)
			return editor, true
		}
	}

	return nil, false
}

// SaveBytesToFile Writes the provided data to a temporary file.
func SaveBytesToFile(f *os.File, d []byte) error {
	err := os.WriteFile(f.Name(), d, config.Files.FilePermissions)
	if err != nil {
		return fmt.Errorf("error writing to temp file: %w", err)
	}

	return nil
}

// CreateTempFileWithData creates a temporary file and writes the provided data
// to it.
func CreateTempFileWithData(d *[]byte) (*os.File, error) {
	const tempExt = "bookmark"
	tf, err := CreateTemp("edit", tempExt)
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	if err := SaveBytesToFile(tf, *d); err != nil {
		return nil, err
	}

	return tf, nil
}

// readContent reads the content of the specified file into the given byte
// slice and returns any error encountered.
func readContent(f *os.File, d *[]byte) error {
	log.Printf("reading file: '%s'", f.Name())
	var err error
	*d, err = os.ReadFile(f.Name())
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	return nil
}

// editFile executes a command to edit the specified file, logging errors if
// the command fails.
func editFile(te *TextEditor, f *os.File) error {
	if te.cmd == "" {
		return ErrCommandNotFound
	}

	log.Printf("editing file: '%s'", f.Name())
	log.Printf("executing args: cmd='%s' args='%v'", te.cmd, te.args)
	if err := sys.RunCmd(te.cmd, append(te.args, f.Name())...); err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}

	return nil
}

// Edit edits the contents of a byte slice by creating a temporary file,
// editing it with an external editor, and then reading the modified contents
// back into the byte slice.
func Edit(te *TextEditor, b *[]byte) error {
	// FIX: im doing too much things or the doc-comm is too long?
	f, err := CreateTempFileWithData(b)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer closeAndClean(f)

	log.Printf("editing file: '%s' with text editor: '%s'", f.Name(), te.name)

	if err := editFile(te, f); err != nil {
		return err
	}

	if err := readContent(f, b); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func newTextEditor(c, n string, arg []string) *TextEditor {
	return &TextEditor{
		cmd:  c,
		name: n,
		args: arg,
	}
}
