package handler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

const maxItemsToEdit = 10

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

// Edition edits the bookmarks using a text editor.
func Edition(r *repo.SQLiteRepository, bs *Slice) error {
	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}
	prompt := fmt.Sprintf("%s %d bookmarks, continue?", color.BrightOrange("editing").Bold(), n)
	if err := confirmUserLimit(n, maxItemsToEdit, prompt); err != nil {
		return err
	}
	te, err := files.GetEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("getting editor: %w", err)
	}
	editFn := func(idx int, b bookmark.Bookmark) error {
		return editBookmark(r, te, &b, idx, n)
	}
	// for each bookmark, invoke the helper to edit it.
	if err := bs.ForEachErrIdx(editFn); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// editBookmark handles editing a single bookmark.
func editBookmark(
	r *repo.SQLiteRepository,
	te *files.TextEditor,
	b *bookmark.Bookmark,
	idx, total int,
) error {
	original := b
	// prepare the buffer with a header and version info.
	buf := prepareBuffer(r, b, idx, total)
	// launch the editor to allow the user to edit the bookmark.
	if err := bookmark.Edit(te, buf, b); err != nil {
		// if nothing was changed, simply continue.
		if errors.Is(err, bookmark.ErrBufferUnchanged) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	return updateBookmark(r, b, original)
}

// prepareBuffer builds the buffer for the bookmark by adding a header and version info.
func prepareBuffer(r *repo.SQLiteRepository, b *bookmark.Bookmark, idx, total int) []byte {
	buf := b.Buffer()
	w := terminal.MinWidth
	const spaces = 7
	// prepare the header with a short title.
	shortTitle := format.Shorten(b.Title, w-10)
	header := fmt.Sprintf("#\n# %d %s %s\n", b.ID, format.BulletPoint, shortTitle)
	// append the header and version information.
	sep := fmt.Sprintf("# %s [%d/%d]\n\n", strings.Repeat("-",
		w-spaces-len(fmt.Sprintf("%d/%d", idx, total))), idx+1, total)
	format.BufferAppend(sep, &buf)
	format.BufferAppend(header, &buf)
	format.BufferAppend(fmt.Sprintf("# database: '%s'\n", r.Cfg.Name), &buf)
	format.BufferAppendVersion(config.App.Name, config.App.Version, &buf)

	return buf
}

// updateBookmark updates the repository with the modified bookmark.
// It calls UpdateURL if the bookmark's URL changed, otherwise it calls Update.
func updateBookmark(r *repo.SQLiteRepository, b, original *bookmark.Bookmark) error {
	if original.URL != b.URL {
		if _, err := r.UpdateURL(b, original); err != nil {
			return fmt.Errorf("updating URL: %w", err)
		}
	} else {
		if _, err := r.Update(b); err != nil {
			return fmt.Errorf("updating bookmark: %w", err)
		}
	}
	fmt.Printf("%s: [%d] %s\n", config.App.Name, b.ID, color.Blue("updated").Bold())

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

		interruptFn := func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}

		t := terminal.New(terminal.WithInterruptFn(interruptFn))
		defer t.CancelInterruptHandler()
		m := menu.New[Bookmark](
			menu.WithInterruptFn(interruptFn),
			menu.WithMultiSelection(),
		)
		prompt := color.BrightRed("remove").Bold().String()
		if err := Confirmation(m, t, bs, prompt, c); err != nil {
			return err
		}
	}

	return removeRecords(r, bs, *force)
}

// removeRecords removes the records from the database.
func removeRecords(r *repo.SQLiteRepository, bs *Slice, force bool) error {
	mesg := color.Gray("removing record/s...").String()
	sp := spinner.New(spinner.WithMesg(mesg))
	sp.Start()

	if err := r.DeleteAndReorder(bs, r.Cfg.Tables.Main, r.Cfg.Tables.Deleted); err != nil {
		return fmt.Errorf("deleting and reordering records: %w", err)
	}

	sp.Stop()

	if !force {
		terminal.ClearLine(1)
	}
	success := color.BrightGreen("Successfully").Italic().String()
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Success(success + " bookmark/s removed").Render()

	return nil
}
