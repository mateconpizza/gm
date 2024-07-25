package editor

import (
	"errors"
	"log"
	"strings"

	"github.com/haaag/gm/pkg/util"
)

var Editor *TextEditor

var (
	ErrBufferEndOfBlock = errors.New("end of the block not found")
	ErrBufferUnchanged  = errors.New("buffer unchanged")
	ErrEditorNotFound   = errors.New("editor not found")
	ErrLineNotFound     = errors.New("not found")
	ErrTooManyRecords   = errors.New("too many records")
)

type TextEditor struct {
	name string
	cmd  string
	args []string
}

func New() *TextEditor {
	return &TextEditor{}
}

// Load retrieves the preferred editor to use for editing
//
// If env variable `EDITOR` is not set, uses the GOMARKS_EDITOR.
// If env variable `GOMARKS_EDITOR` is not set, uses the first available
// `TextEditors`
//
// # fallbackEditors: `"vim", "nvim", "nano", "emacs", "helix"`.
func Load(env *string, fallbackEditors *[]string) error {
	Editor = New()
	s := strings.Fields(util.GetEnv(*env, "EDITOR"))
	if len(s) != 0 {
		Editor.name = s[0]
		Editor.args = s[1:]
		Editor.cmd = util.BinPath(s[0])
		log.Printf("$EDITOR set: '%v'", Editor)

		return nil
	}

	log.Printf("$EDITOR not set, checking fallback text editor: %s", *fallbackEditors)
	for _, e := range *fallbackEditors {
		if util.BinExists(e) {
			Editor.name = e
			Editor.cmd = util.BinPath(e)
			log.Printf("found fallback text editor: '%v'", Editor)

			return nil
		}
	}

	return ErrEditorNotFound
}
