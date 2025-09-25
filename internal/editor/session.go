package editor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/db"
)

// EditSession build -> edit -> parse -> confirm -> save.
type EditSession struct {
	Console *ui.Console
	Editor  *TextEditor
	DB      *db.SQLite
}

func (e *EditSession) Run(bs []*Record, strategy EditStrategy) error {
	total := len(bs)

	for idx, original := range bs {
	out:
		for {
			// Build buffer for current bookmark
			buf, err := strategy.BuildBuffer(original, idx, total)
			if err != nil {
				return err
			}

			// Edit buffer
			editedBuf, err := e.Editor.Bytes(buf, strategy.EditType())
			if err != nil {
				return err
			}

			// Parse updated bookmark
			updated, err := strategy.ParseBuffer(editedBuf, original, idx, total)
			if errors.Is(err, ErrBufferUnchanged) {
				break out // nothing changed
			}
			if err != nil {
				return err
			}

			// Show diff
			e.Console.F.Reset().Header("Diff:\n").Flush()
			fmt.Println(strategy.Diff(original, updated))

			// Confirm action
			opt, err := e.Console.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
			if err != nil {
				return err
			}

			switch strings.ToLower(opt) {
			case "y", "yes":
				if err := strategy.Save(context.Background(), e.DB, updated); err != nil {
					return err
				}
				fmt.Print(e.Console.SuccessMesg(fmt.Sprintf("bookmark [%d] changes saved\n", updated.ID)))
				break out

			case "n", "no":
				break out // Skip and continue

			case "e", "edit":
				original = updated // Retry
			}
		}
	}
	return nil
}

func NewEditSession(c *ui.Console, e *TextEditor, r *db.SQLite) *EditSession {
	return &EditSession{
		Console: c,
		Editor:  e,
		DB:      r,
	}
}
