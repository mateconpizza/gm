package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/rotato"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/menu"
	"github.com/mateconpizza/gm/internal/repo"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

const maxItemsToEdit = 10

// Records gets records based on user input and filtering criteria.
func Records(r *repo.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark], args []string) error {
	slog.Debug("records", "args", args)
	if err := ByIDs(r, bs, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := ByQuery(r, bs, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if bs.Empty() && len(args) == 0 {
		// if empty, get all records
		bb, err := r.All()
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		bs.Set(&bb)
	}

	return nil
}

// Data processes records based on user input and filtering criteria.
func Data(
	cmd *cobra.Command,
	m *menu.Menu[bookmark.Bookmark],
	r *repo.SQLiteRepository,
	args []string,
) (*slice.Slice[bookmark.Bookmark], error) {
	bs := slice.New[bookmark.Bookmark]()
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
		items, err := selectionWithMenu(m, *bs.Items(), fzfFormatter(mlFlag))
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
	buf := prepareBuffer(b, idx, total)

	f := frame.New(frame.WithColorBorder(color.BrightGray))

	// launch the editor to allow the user to edit the bookmark.
	if err := bookmark.Edit(te, t, f, buf, b); err != nil {
		if errors.Is(err, bookmark.ErrBufferUnchanged) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	return updateBookmark(r, b, &originalData)
}

// prepareBuffer builds the buffer for the bookmark by adding a header and version info.
func prepareBuffer(b *bookmark.Bookmark, idx, total int) []byte {
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
	format.BufferAppend(fmt.Sprintf("# database:\t%q\n", config.App.DBName), &buf)
	format.BufferAppend(fmt.Sprintf("# %s:\tv%s\n", "version", config.App.Info.Version), &buf)
	format.BufferAppend(header, &buf)
	format.BufferAppendEnd(fmt.Sprintf(" [%d/%d]", idx+1, total), &buf)

	return buf
}

// updateBookmark updates the repository with the modified bookmark.
func updateBookmark(r *repo.SQLiteRepository, b, original *bookmark.Bookmark) error {
	if _, err := r.Update(context.Background(), b, original); err != nil {
		return fmt.Errorf("updating record: %w", err)
	}
	fmt.Printf("%s: [%d] %s\n", config.App.Name, b.ID, color.Blue("updated").Bold())

	// FIX: find a better way to remove
	// old gpg file
	if gpg.IsInitialized(config.App.Path.Git) {
		root := filepath.Join(config.App.Path.Git, files.StripSuffixes(r.Cfg.Name))
		if err := bookmark.CleanupGitFiles(root, original, ".gpg"); err != nil {
			return fmt.Errorf("cleaning up git files: %w", err)
		}
	}
	return GitCommit("Modify")
}

// removeRecords removes the records from the database.
func removeRecords(r *repo.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
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

	// remove GPG file
	root := config.App.Path.Git
	if gpg.IsInitialized(root) {
		for _, b := range bs.ItemsPtr() {
			root := filepath.Join(config.App.Path.Git, files.StripSuffixes(r.Cfg.Name))
			if err := bookmark.CleanupGitFiles(root, b, ".gpg"); err != nil {
				return fmt.Errorf("cleaning up git files: %w", err)
			}
		}
		if err := GitCommit("Remove"); err != nil {
			return err
		}
	}

	success := color.BrightGreen("Successfully").Italic().String()
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Success(success + " bookmark/s removed\n").Flush()

	return nil
}

// diffDeletedBookmarks checks for deleted bookmarks.
func diffDeletedBookmarks(root string, bookmarks []*bookmark.Bookmark) error {
	jsonBookmarks := slice.New[bookmark.Bookmark]()
	if err := bookmark.LoadJSONBookmarks(root, jsonBookmarks); err != nil {
		return fmt.Errorf("loading JSON bookmarks: %w", err)
	}
	diff := bookmark.FindChanged(bookmarks, jsonBookmarks.ItemsPtr())
	if len(diff) == 0 {
		return nil
	}

	for _, b := range diff {
		if _, err := repo.HasURL(b.URL); err != nil {
			continue
		}
		if err := bookmark.CleanupGitFiles(root, b, ".json"); err != nil {
			return fmt.Errorf("cleanup files: %w", err)
		}
	}
	return nil
}

// FindDB returns the path to the database.
func FindDB(p string) (string, error) {
	slog.Debug("searching db", "path", p)
	if files.Exists(p) {
		return p, nil
	}
	fs, err := repo.Databases(filepath.Dir(p))
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	s := filepath.Base(p)
	for _, f := range fs {
		if strings.Contains(f, s) {
			return f, nil
		}
	}

	return "", fmt.Errorf("%w: %q", repo.ErrDBNotFound, files.StripSuffixes(s))
}
