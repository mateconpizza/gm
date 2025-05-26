package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/haaag/rotato"
	"github.com/spf13/cobra"

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
	"github.com/haaag/gm/internal/sys/terminal"
)

const maxItemsToEdit = 10

type (
	Bookmark = bookmark.Bookmark
	Slice    = slice.Slice[Bookmark]
)

// Records gets records based on user input and filtering criteria.
func Records(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	slog.Debug("records", "args", args)
	if err := ByIDs(r, bs, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := ByQuery(r, bs, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if bs.Empty() && len(args) == 0 {
		// if empty, get all records
		if err := r.All(bs); err != nil {
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
	te, err := files.NewEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("getting editor: %w", err)
	}
	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	}))
	editFn := func(idx int, b bookmark.Bookmark) error {
		return editBookmark(r, te, t, &b, idx, n)
	}
	// for each bookmark, invoke the helper to edit it.
	if err := bs.ForEachIdxErr(editFn); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// Data processes records based on user input and filtering criteria.
func Data(
	cmd *cobra.Command,
	m *menu.Menu[Bookmark],
	r *repo.SQLiteRepository,
	args []string,
) (*Slice, error) {
	bs := slice.New[Bookmark]()
	if err := Records(r, bs, args); err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	// filter by Tag
	tags, err := cmd.Flags().GetStringSlice("tag")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if len(tags) > 0 {
		if err := ByTags(r, tags, bs); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}
	// filter by head and tail
	head, err := cmd.Flags().GetInt("head")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	tail, err := cmd.Flags().GetInt("tail")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if head > 0 || tail > 0 {
		if err := ByHeadAndTail(bs, head, tail); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}
	// select with fzf-menu
	mFlag, err := cmd.Flags().GetBool("menu")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	mlFlag, err := cmd.Flags().GetBool("multiline")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if mFlag || mlFlag {
		items, err := SelectionWithMenu(m, *bs.Items(), fzfFormatter(mlFlag))
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		bs.Set(&items)
	}

	return bs, nil
}

// editBookmark handles editing a single bookmark.
func editBookmark(
	r *repo.SQLiteRepository,
	te *files.TextEditor,
	t *terminal.Term,
	b *bookmark.Bookmark,
	idx, total int,
) error {
	originalData := *b
	// prepare the buffer with a header and version info.
	buf := prepareBuffer(r, b, idx, total)
	// launch the editor to allow the user to edit the bookmark.
	if err := bookmark.Edit(te, t, buf, b); err != nil {
		if errors.Is(err, bookmark.ErrBufferUnchanged) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	return updateBookmark(r, b, &originalData)
}

// prepareBuffer builds the buffer for the bookmark by adding a header and version info.
func prepareBuffer(r *repo.SQLiteRepository, b *bookmark.Bookmark, idx, total int) []byte {
	buf := b.Buffer()
	w := terminal.MinWidth
	const spaces = 10
	// prepare the header with a short title.
	shortTitle := format.Shorten(b.Title, w-spaces-6)
	header := fmt.Sprintf("# %d %s\n", b.ID, shortTitle)
	header += "#\n"
	// append the header and version information.
	sep := format.CenteredLine(terminal.MinWidth-spaces, "bookmark edition")
	format.BufferAppend("# "+sep+"\n\n", &buf)
	format.BufferAppend(fmt.Sprintf("# database:\t%q\n", r.Name()), &buf)
	format.BufferAppend(fmt.Sprintf("# %s:\tv%s\n", "version", config.App.Version), &buf)
	format.BufferAppend(header, &buf)
	format.BufferAppendEnd(fmt.Sprintf(" [%d/%d]", idx+1, total), &buf)

	return buf
}

// updateBookmark updates the repository with the modified bookmark.
// It calls UpdateURL if the bookmark's URL changed, otherwise it calls Update.
func updateBookmark(r *repo.SQLiteRepository, b, original *bookmark.Bookmark) error {
	ctx := context.Background()
	if _, err := r.UpdateOne(ctx, b, original); err != nil {
		return fmt.Errorf("updating record: %w", err)
	}
	fmt.Printf("%s: [%d] %s\n", config.App.Name, b.ID, color.Blue("updated").Bold())

	return nil
}

// removeRecords removes the records from the database.
func removeRecords(r *repo.SQLiteRepository, bs *Slice) error {
	sp := rotato.New(
		rotato.WithMesg("removing record/s..."),
		rotato.WithMesgColor(rotato.ColorGray),
	)
	sp.Start()
	ctx := context.Background()
	// delete records from main table.
	if err := r.DeleteMany(ctx, bs); err != nil {
		return fmt.Errorf("deleting records: %w", err)
	}
	// reorder IDs from main table to avoid gaps.
	if err := r.ReorderIDs(ctx); err != nil {
		return fmt.Errorf("reordering IDs: %w", err)
	}
	// recover space after deletion.
	if err := r.Vacuum(); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Done()

	if !config.App.Force {
		terminal.ClearLine(1)
	}
	success := color.BrightGreen("Successfully").Italic().String()
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Success(success + " bookmark/s removed\n").Flush()

	return nil
}

// insertRecordsToRepo inserts records into the database.
func insertRecordsToRepo(t *terminal.Term, r *repo.SQLiteRepository, records *Slice) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if !config.App.Force {
		report := fmt.Sprintf("import %d records?", records.Len())
		if err := t.ConfirmErr(f.Row("\n").Question(report).String(), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}
	sp := rotato.New(
		rotato.WithMesg("importing record/s..."),
		rotato.WithMesgColor(rotato.ColorYellow),
	)
	sp.Start()
	if err := r.InsertMany(context.Background(), records); err != nil {
		return fmt.Errorf("%w", err)
	}
	sp.Done()
	success := color.BrightGreen("Successfully").Italic().String()
	msg := fmt.Sprintf(success+" imported %d record/s\n", records.Len())
	f.Clear().Success(msg).Flush()

	return nil
}
