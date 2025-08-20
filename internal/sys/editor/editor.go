package editor

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/pkg/files"
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

// EditBytes edits a byte slice with a text editor.
func (te *TextEditor) EditBytes(content []byte, extension string) ([]byte, error) {
	if te.cmd == "" {
		return nil, ErrCommandNotFound
	}

	f, err := files.CreateTempFileWithData(content, extension)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	defer files.CloseAndClean(f)

	slog.Debug("editing file", "name", f.Name(), "editor", te.name)

	if err := sys.RunCmd(te.cmd, append(te.args, f.Name())...); err != nil {
		return nil, fmt.Errorf("error running editor: %w", err)
	}

	data, err := os.ReadFile(f.Name())
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return data, nil
}

// EditFile edits a file with a text editor.
func (te *TextEditor) EditFile(p string) error {
	if te.cmd == "" {
		return ErrCommandNotFound
	}

	if !files.Exists(p) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, p)
	}

	if err := sys.RunCmd(te.cmd, append(te.args, p)...); err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}

	return nil
}

// New retrieves the preferred editor to use for editing
//
// If env variable `GOMARKS_EDITOR` is not set, uses the `EDITOR`.
// If env variable `EDITOR` is not set, uses the first available
// `TextEditors`
//
// # fallbackEditors: `"vim", "nvim", "nano", "emacs"`.
func New(s string) (*TextEditor, error) {
	envs := []string{s, "EDITOR"}
	// find $EDITOR and $GOMARKS_EDITOR
	for _, e := range envs {
		if editor, found := getEditorFromEnv(e); found {
			if editor.cmd == "" {
				return nil, fmt.Errorf("%w: %q", ErrTextEditorNotFound, editor.name)
			}

			return editor, nil
		}
	}

	slog.Debug(
		"$EDITOR and $GOMARKS_EDITOR not set, checking fallback text editor",
		"editors", textEditors,
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
		slog.Info("$EDITOR set", "editor", editor)

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
			slog.Info("found fallback text editor", "editor", editor)

			return editor, true
		}
	}

	return nil, false
}

func newTextEditor(c, n string, arg []string) *TextEditor {
	return &TextEditor{
		cmd:  c,
		name: n,
		args: arg,
	}
}
