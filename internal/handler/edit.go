package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/parser"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/editor"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// FIX: extract same logic from `EditNotes` `EditBookmarks`. DRY

type BookmarkEditor struct {
	Console *ui.Console
	DB      *db.SQLite
	Editor  *editor.TextEditor
}

//nolint:funlen //ignore
func (be *BookmarkEditor) EditNotes(bs []*bookmark.Bookmark) error {
	const spaces = 4
	total := len(bs)

	// metadata
	sep := txt.CenteredLine(terminal.MinWidth-spaces, "bookmark notes")
	meta := fmt.Appendf(nil,
		"# database:\t%q\n# version:\tv%s\n# %s\n\n",
		config.App.DBName,
		config.App.Info.Version,
		sep,
	)

	// indentation
	idLen := 4
	n := terminal.MinWidth - spaces

	for i := range bs {
		b := bs[i]
		current := *b

	out:
		for {
			editOpts := parser.NewBookmarkEditOps(&current)
			editOpts.Idx = i
			editOpts.Total = total
			editOpts.Body = current.BufferNotes()

			titleSplit := txt.SplitIntoChunks(current.Title, n-idLen)
			shortTitle := strings.Join(titleSplit, "\n# ")
			shortTitle += "\n#\n# " + txt.Shorten(current.URL, n)
			header := fmt.Appendf(nil, "# %d %s\n#\n", current.ID, shortTitle)

			editOpts.Header = append(editOpts.Header, header...)
			editOpts.Header = append(editOpts.Header, meta...)

			original := []byte(current.Notes)

			// open editor
			edited, err := be.Editor.EditBytes(editOpts.Buffer(), config.App.Name)
			if err != nil {
				return fmt.Errorf("edit notes: %w", err)
			}

			// unchanged? skip this bookmark
			editedNotesBytes := txt.ExtractBlockBytes(edited, "# Notes", "")
			if bytes.Equal(original, editedNotesBytes) {
				break out
			}

			// show diff
			be.Console.F.Reset().Header(cy("Edit Notes:\n")).Flush()
			diff := txt.Diff(original, editedNotesBytes)
			fmt.Println(txt.DiffColor(diff))

			opt, err := be.Console.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
			if err != nil {
				return fmt.Errorf("choose: %w", err)
			}

			newNotes := string(editedNotesBytes)

			switch strings.ToLower(opt) {
			case "y", "yes":
				// parse new notes, update DB
				current.Notes = newNotes
				if err := be.DB.UpdateOne(context.Background(), &current); err != nil {
					return fmt.Errorf("update notes: %w", err)
				}
				fmt.Print(be.Console.SuccessMesg("notes updated\n"))
				break out
			case "n", "no":
				break out // skip and continue with next
			case "e", "edit":
				// keep current edited buffer for another round
				current.Notes = newNotes
			}
		}
	}

	return nil
}

func (be *BookmarkEditor) EditJSON(bs []*bookmark.Bookmark) error {
	for i := range bs {
		b := bs[i]
		oldB := b.Bytes()

	out:
		for {
			newB, err := be.Editor.EditBytes(oldB, "json")
			if err != nil {
				return err
			}

			oldB = bytes.TrimRight(oldB, "\n")
			newB = bytes.TrimRight(newB, "\n")
			if bytes.Equal(newB, oldB) {
				break out
			}

			diff := txt.Diff(oldB, newB)
			fmt.Println(txt.DiffColor(diff))
			opt, err := be.Console.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
			if err != nil {
				return fmt.Errorf("choose: %w", err)
			}

			switch strings.ToLower(opt) {
			case "y", "yes":
				bm, err := bookmark.NewFromBuffer(newB)
				if err != nil {
					return err
				}

				if err := be.DB.UpdateOne(context.Background(), bm); err != nil {
					return fmt.Errorf("update: %w", err)
				}

				fmt.Print(be.Console.SuccessMesg("bookmark updated\n"))

				break out
			case "n", "no":
				return sys.ErrActionAborted
			case "e", "edit":
				oldB = newB
			}
		}
	}

	return nil
}

func (be *BookmarkEditor) EditBookmarks(bs []*bookmark.Bookmark) error {
	n := len(bs)

	for i := range bs {
		b := bs[i]
		current := *b

	out:
		for {
			editedB, err := parser.Edit(be.Editor, &current, i, n)
			if err != nil {
				if errors.Is(err, parser.ErrBufferUnchanged) {
					break out
				}
				return fmt.Errorf("edit: %w", err)
			}

			be.Console.F.Reset().Header(cy("Edit Bookmark:\n")).Flush()
			diff := txt.Diff(current.Buffer(), editedB.Buffer())
			fmt.Println(txt.DiffColor(diff))

			opt, err := be.Console.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
			if err != nil {
				return fmt.Errorf("choose: %w", err)
			}

			// update fields from editedB
			b.URL = editedB.URL
			b.Title = editedB.Title
			b.Desc = editedB.Desc
			b.Tags = editedB.Tags
			b.FaviconURL = editedB.FaviconURL

			switch strings.ToLower(opt) {
			case "y", "yes":
				if err := handleEditedBookmark(be.Console, be.DB, b, editedB); err != nil {
					return err
				}
				break out // go to next bookmark
			case "n", "no":
				break out // skip current, continue with next
			case "e", "edit":
				current = *editedB // retry editing same bookmark
			}
		}
	}

	return nil
}

func NewBookmarkEditor(c *ui.Console, e *editor.TextEditor, r *db.SQLite) *BookmarkEditor {
	return &BookmarkEditor{
		Console: c,
		Editor:  e,
		DB:      r,
	}
}
