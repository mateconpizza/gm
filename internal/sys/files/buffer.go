package files

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/haaag/gm/internal/sys"
)

var (
	ErrCommandNotFound    = errors.New("command not found")
	ErrTextEditorNotFound = errors.New("text editor not found")
)

const (
	dirPerm  = 0o755 // Permissions for new directories.
	filePerm = 0o644 // Permissions for new files.
)

// Fallback text editors if $EDITOR || $GOMARKS_EDITOR var is not set.
var textEditors = []string{"vim", "nvim", "nano", "emacs"}

type TextEditor struct {
	name string
	cmd  string
	args []string
}

// EditBytes edits a byte slice with a text editor.
func (te *TextEditor) EditBytes(content []byte) ([]byte, error) {
	if te.cmd == "" {
		return nil, ErrCommandNotFound
	}
	f, err := createTEmpFileWithData(content)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	defer closeAndClean(f)

	log.Printf("editing file: %q with text editor: %q", f.Name(), te.name)
	log.Printf("executing args: cmd=%q args='%v'", te.cmd, te.args)
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

	if !Exists(p) {
		return fmt.Errorf("%w: %q", ErrFileNotFound, p)
	}

	if err := sys.RunCmd(te.cmd, append(te.args, p)...); err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}

	return nil
}

// Diff Take two []byte and return a string with the complete diff.
func (te *TextEditor) Diff(a, b []byte) string {
	linesA := strings.Split(string(a), "\n")
	linesB := strings.Split(string(b), "\n")
	m, n := len(linesA), len(linesB)

	// create the matrix for LCS (Longest Common Subsequence).
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	// fill the DP (Dynamic Programming) matrix with the length of the LCS.
	// dp[i+1][j+1] stores the length of the LCS between linesA[:i+1] and linesB[:j+1].
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			if linesA[i] == linesB[j] {
				// if lines match, LCS length increases by 1.
				dp[i+1][j+1] = dp[i][j] + 1
			} else {
				// otherwise, take the maximum value from the previous row or column.
				if dp[i+1][j] >= dp[i][j+1] {
					dp[i+1][j+1] = dp[i+1][j]
				} else {
					dp[i+1][j+1] = dp[i][j+1]
				}
			}
		}
	}

	// backtrack to construct the diff output.
	var diffLines []string
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && linesA[i-1] == linesB[j-1]:
			// unchanged (common line)
			diffLines = append([]string{linesA[i-1]}, diffLines...)
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			// added
			diffLines = append([]string{"+" + linesB[j-1]}, diffLines...)
			j--
		case i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]):
			// deleted
			diffLines = append([]string{"-" + linesA[i-1]}, diffLines...)
			i--
		}
	}

	return strings.Join(diffLines, "\n")
}

// NewEditor retrieves the preferred editor to use for editing
//
// If env variable `GOMARKS_EDITOR` is not set, uses the `EDITOR`.
// If env variable `EDITOR` is not set, uses the first available
// `TextEditors`
//
// # fallbackEditors: `"vim", "nvim", "nano", "emacs"`.
func NewEditor(s string) (*TextEditor, error) {
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

// saveBytestToFile Writes the provided data to a temporary file.
func saveBytestToFile(f *os.File, d []byte) error {
	err := os.WriteFile(f.Name(), d, filePerm)
	if err != nil {
		return fmt.Errorf("error writing to temp file: %w", err)
	}

	return nil
}

// createTEmpFileWithData creates a temporary file and writes the provided data
// to it.
func createTEmpFileWithData(d []byte) (*os.File, error) {
	const tempExt = "bookmark"
	tf, err := CreateTemp("edit", tempExt)
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}

	if err := saveBytestToFile(tf, d); err != nil {
		return nil, err
	}

	return tf, nil
}

func newTextEditor(c, n string, arg []string) *TextEditor {
	return &TextEditor{
		cmd:  c,
		name: n,
		args: arg,
	}
}
