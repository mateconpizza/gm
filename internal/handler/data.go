package handler

import (
	"errors"
	"fmt"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

type (
	Bookmark = bookmark.Bookmark
	Slice    = slice.Slice[Bookmark]
)

// Records gets records based on user input and filtering criteria.
func Records(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	if err := ByIDs(r, bs, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := ByQuery(r, bs, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if bs.Empty() && len(args) == 0 {
		// if empty, get all records
		if err := r.Records(r.Cfg.Tables.Main, bs); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// Edition edits the bookmark with a text editor.
func Edition(r *repo.SQLiteRepository, bs *Slice) error {
	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	const maxItems = 10
	e := color.BrightOrange("editing").Bold()
	q := fmt.Sprintf("%s %d bookmarks, continue?", e, n)
	if err := confirmUserLimit(n, maxItems, q); err != nil {
		return err
	}

	header := "# [%d/%d] | %d | %s\n\n"
	te, err := files.Editor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// edition edits the bookmark with a text editor.
	edition := func(i int, b Bookmark) error {
		tempB := b

		// prepare header and buffer
		buf := bookmark.Buffer(&b)
		tShort := format.Shorten(b.Title, terminal.MinWidth-10)
		format.BufferAppend(fmt.Sprintf(header, i+1, n, b.ID, tShort), &buf)
		format.BufferAppendVersion(config.App.Name, config.App.Version, &buf)
		bufCopy := make([]byte, len(buf))
		copy(bufCopy, buf)

		if err := bookmark.Edit(te, buf, &b); err != nil {
			if errors.Is(err, bookmark.ErrBufferUnchanged) {
				return nil
			}

			return fmt.Errorf("%w", err)
		}

		// FIX: find a better way to update URL
		if tempB.URL != b.URL {
			if _, err := r.UpdateURL(&b, &tempB); err != nil {
				return fmt.Errorf("updating URL: %w", err)
			}
		} else {
			if _, err := r.Update(&b); err != nil {
				return fmt.Errorf("handle edition: %w", err)
			}
		}

		fmt.Printf("%s: [%d] %s\n", config.App.Name, b.ID, color.Blue("updated").Bold())

		return nil
	}

	if err := bs.ForEachErrIdx(edition); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// Remove prompts the user the records to remove.
func Remove(r *repo.SQLiteRepository, bs *Slice) error {
	defer r.Close()

	if err := validateRemove(bs, *force); err != nil {
		return err
	}

	if !*force {
		c := color.BrightRed
		f := frame.New(frame.WithColorBorder(c), frame.WithNoNewLine())
		header := c("Removing Bookmarks").String()
		f.Header(header).Ln().Ln().Render().Clean()

		prompt := color.BrightRed("remove").Bold().String()
		if err := Confirmation(bs, prompt, c); err != nil {
			return err
		}
	}

	return removeRecords(r, bs, *force)
}

// removeRecords removes the records from the database.
func removeRecords(r *repo.SQLiteRepository, bs *Slice, force bool) error {
	mesg := color.Gray("removing record/s...").String()
	s := spinner.New(spinner.WithMesg(mesg))
	s.Start()

	if err := r.DeleteAndReorder(bs, r.Cfg.Tables.Main, r.Cfg.Tables.Deleted); err != nil {
		return fmt.Errorf("deleting and reordering records: %w", err)
	}

	s.Stop()

	if !force {
		terminal.ClearLine(1)
	}
	success := color.BrightGreen("Successfully").Italic().String()
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Success(success + " bookmark/s removed").Render()

	return nil
}
