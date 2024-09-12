package files

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/util"
)

var (
	ErrBufferUnchanged    = errors.New("buffer unchanged")
	ErrCommandNotFound    = errors.New("command not found")
	ErrTextEditorNotFound = errors.New("text editor not found")
)

// Fallback text editors if $EDITOR || $GOMARKS_EDITOR var is not set.
var textEditors = []string{"vim", "nvim", "nano", "emacs"}

type TextEditor struct {
	Name string
	Cmd  string
	Args []string
}

func newTextEditor(c, n string, a []string) *TextEditor {
	return &TextEditor{
		Cmd:  c,
		Name: n,
		Args: a,
	}
}

// GetEditor retrieves the preferred editor to use for editing
//
// If env variable `GOMARKS_EDITOR` is not set, uses the `EDITOR`.
// If env variable `EDITOR` is not set, uses the first available
// `TextEditors`
//
// # fallbackEditors: `"vim", "nvim", "nano", "emacs"`.
func GetEditor(env string) (*TextEditor, error) {
	envs := []string{env, "EDITOR"}

	// find $EDITOR and $GOMARKS_EDITOR
	for _, e := range envs {
		if editor, found := getEditorFromEnv(e); found {
			if editor.Cmd == "" {
				return nil, fmt.Errorf("%w: '%s'", ErrTextEditorNotFound, editor.Name)
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
func getEditorFromEnv(env string) (*TextEditor, bool) {
	s := strings.Fields(util.GetEnv(env, ""))
	if len(s) != 0 {
		editor := newTextEditor(util.BinPath(s[0]), s[0], s[1:])
		log.Printf("$EDITOR set: '%v'", editor)
		return editor, true
	}

	return nil, false
}

// getFallbackEditor finds a fallback editor.
func getFallbackEditor(editors []string) (*TextEditor, bool) {
	for _, e := range editors {
		if util.BinExists(e) {
			editor := newTextEditor(util.BinPath(e), e, []string{})
			log.Printf("found fallback text editor: '%v'", editor)
			return editor, true
		}
	}

	return nil, false
}

// SaveBytesToFile Writes the provided data to a temporary file.
func SaveBytesToFile(f *os.File, data []byte) error {
	err := os.WriteFile(f.Name(), data, config.Files.FilePermissions)
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

// ReadFileContent reads the content of the specified file into the given byte
// slice and returns any error encountered.
func ReadFileContent(fileName *os.File, data *[]byte) error {
	log.Printf("reading file: '%s'", fileName.Name())
	var err error
	*data, err = os.ReadFile(fileName.Name())
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	return nil
}

// editFile executes a command to edit the specified file, logging errors if
// the command fails.
func editFile(fileName *os.File, command string, args []string) error {
	if command == "" {
		return ErrCommandNotFound
	}

	log.Printf("editing file: '%s'", fileName.Name())
	log.Printf("executing args: cmd='%s' args='%v'", command, args)
	if err := util.RunCmd(command, append(args, fileName.Name())...); err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}

	return nil
}

// Edit edits the contents of a byte slice by creating a temporary file,
// editing it with an external editor, and then reading the modified contents
// back into the byte slice.
func Edit(editor *TextEditor, buf *[]byte) error {
	tf, err := CreateTempFileWithData(buf)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer Cleanup(tf)

	log.Printf("editing file: '%s' with text editor: '%s'", tf.Name(), editor.Name)

	if err := editFile(tf, editor.Cmd, editor.Args); err != nil {
		return err
	}

	if err := ReadFileContent(tf, buf); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
