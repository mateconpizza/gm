package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/rotato"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

var (
	ErrInvalidOption = errors.New("invalid option")
	ErrNoItems       = errors.New("no items")
)

// records gets records based on user input and filtering criteria.
func records(r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark], args []string) error {
	slog.Debug("records", "args", args)
	if err := byIDs(r, bs, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := byQuery(r, bs, args); err != nil {
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
	r *db.SQLiteRepository,
	args []string,
) (*slice.Slice[bookmark.Bookmark], error) {
	bs := slice.New[bookmark.Bookmark]()
	if err := records(r, bs, args); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// filter by Tag
	tags, err := cmd.Flags().GetStringSlice("tag")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if len(tags) > 0 {
		if err := byTags(r, tags, bs); err != nil {
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
		if err := headAndTail(bs, head, tail); err != nil {
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

func handleEditedBookmark(r *db.SQLiteRepository, newB, oldB *bookmark.Bookmark) error {
	newBookmark := newB.ID == 0
	if newBookmark {
		return addBookmark(r, newB)
	}

	return updateBookmark(r, newB, oldB)
}

// addBookmark adds a new bookmark.
func addBookmark(r *db.SQLiteRepository, b *bookmark.Bookmark) error {
	if err := r.InsertOne(context.Background(), b); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := port.GitStore(b); err != nil {
		return fmt.Errorf("git store: %w", err)
	}

	if err := GitCommit(r.Cfg.Fullpath(), config.App.Path.Git, "Add"); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(txt.SuccessMesg("bookmark added"))

	return nil
}

// updateBookmark updates the repository with the modified bookmark.
func updateBookmark(r *db.SQLiteRepository, newB, oldB *bookmark.Bookmark) error {
	if _, err := r.Update(context.Background(), newB, oldB); err != nil {
		return fmt.Errorf("updating record: %w", err)
	}

	if err := port.GitUpdate(r.Cfg.Fullpath(), newB, oldB); err != nil {
		return fmt.Errorf("git update: %w", err)
	}

	fmt.Print(txt.SuccessMesg(fmt.Sprintf("bookmark [%d] updated", newB.ID)))

	return GitCommit(r.Cfg.Fullpath(), config.App.Path.Git, "Modify")
}

// removeRecords removes the records from the database.
func removeRecords(r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
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

	if err := gitCleanFiles(config.App.Path.Git, r, bs); err != nil {
		return err
	}

	fmt.Print(txt.SuccessMesg("bookmark/s removed\n"))

	return nil
}

// FindDB returns the path to the database.
func FindDB(p string) (string, error) {
	slog.Debug("searching db", "path", p)
	if files.Exists(p) {
		return p, nil
	}
	fs, err := db.List(filepath.Dir(p))
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	s := filepath.Base(p)
	for _, f := range fs {
		if strings.Contains(f, s) {
			return f, nil
		}
	}

	return "", fmt.Errorf("%w: %q", db.ErrDBNotFound, files.StripSuffixes(s))
}

// byTags returns a slice of bookmarks based on the provided tags.
func byTags(r *db.SQLiteRepository, tags []string, bs *slice.Slice[bookmark.Bookmark]) error {
	slog.Debug("by tags", "tags", tags, "count", bs.Len())
	// FIX: redo, simplify
	// if the slice contains bookmarks, filter by tag.
	if !bs.Empty() {
		for _, tag := range tags {
			bs.FilterInPlace(func(b *bookmark.Bookmark) bool {
				return strings.Contains(b.Tags, tag)
			})
		}

		return nil
	}

	for _, tag := range tags {
		bb, err := r.ByTag(context.Background(), tag)
		if err != nil {
			return fmt.Errorf("byTags :%w", err)
		}
		bs.Append(bb...)
	}

	if bs.Empty() {
		t := strings.Join(tags, ", ")
		return fmt.Errorf("%w by tag: %q", db.ErrRecordNoMatch, t)
	}

	bs.FilterInPlace(func(b *bookmark.Bookmark) bool {
		for _, tag := range tags {
			if !strings.Contains(b.Tags, tag) {
				return false
			}
		}

		return true
	})

	return nil
}

// byQuery executes a search query on the given repository based on provided
// arguments.
func byQuery(r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark], args []string) error {
	// FIX: do i need this?
	if bs.Len() != 0 || len(args) == 0 {
		return nil
	}

	q := strings.Join(args, "%")
	bb, err := r.ByQuery(q)
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.Join(args, " "))
	}
	bs.Set(&bb)

	return nil
}

// byIDs retrieves records from the database based on either
// an ID or a query string.
func byIDs(r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark], args []string) error {
	slog.Debug("getting by IDs")
	ids, err := extractIDsFrom(args)
	if len(ids) == 0 {
		return nil
	}

	if !errors.Is(err, bookmark.ErrInvalidID) && err != nil {
		return fmt.Errorf("%w", err)
	}

	bb, err := r.ByIDList(ids)
	if err != nil {
		return fmt.Errorf("records from args: %w", err)
	}
	bs.Set(&bb)

	if bs.Empty() {
		bids := strings.TrimRight(strings.Join(args, ", "), "\n")
		return fmt.Errorf("%w by id/s: %s in %q", db.ErrRecordNotFound, bids, r.Name())
	}

	return nil
}

// headAndTail returns a slice of bookmarks with limited elements.
func headAndTail(bs *slice.Slice[bookmark.Bookmark], h, t int) error {
	if h == 0 && t == 0 {
		return nil
	}
	if h < 0 || t < 0 {
		return fmt.Errorf("%w: head=%d tail=%d", ErrInvalidOption, h, t)
	}

	// determine flag order
	rawArgs := os.Args[1:]
	order := []string{}
	for _, arg := range rawArgs {
		if strings.HasPrefix(arg, "-H") || strings.HasPrefix(arg, "--head") {
			order = append(order, "head")
		} else if strings.HasPrefix(arg, "-T") || strings.HasPrefix(arg, "--tail") {
			order = append(order, "tail")
		}
	}

	for _, action := range order {
		switch action {
		case "head":
			if h > 0 {
				bs.Head(h)
			}
		case "tail":
			if t > 0 {
				bs.Tail(t)
			}
		}
	}

	return nil
}
