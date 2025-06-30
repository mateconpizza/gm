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

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

var (
	ErrInvalidOption = errors.New("invalid option")
	ErrNoItems       = errors.New("no items")
)

// records gets records based on user input and filtering criteria.
func records(r *db.SQLite, bs *slice.Slice[bookmark.Bookmark], args []string) error {
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
	m *menu.Menu[bookmark.Bookmark],
	r *db.SQLite,
	args []string,
) ([]*bookmark.Bookmark, error) {
	f := config.App.Flags
	bs := slice.New[bookmark.Bookmark]()
	if err := records(r, bs, args); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// filter by Tag
	if len(f.Tags) > 0 {
		if err := byTags(r, f.Tags, bs); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}

	// filter by head and tail
	if f.Head > 0 || f.Tail > 0 {
		if err := headAndTail(bs, f.Head, f.Tail); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}

	// select with fzf-menu
	if f.Menu || f.Multiline {
		items, err := selectionWithMenu(m, *bs.Items(), func(b *bookmark.Bookmark) string {
			if f.Multiline {
				return bookmark.Multiline(b)
			}

			return bookmark.Oneline(b)
		})
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		bs.Set(&items)
	}

	return bs.ItemsPtr(), nil
}

func handleEditedBookmark(c *ui.Console, r *db.SQLite, newB, oldB *bookmark.Bookmark) error {
	newBookmark := newB.ID == 0
	if newBookmark {
		return r.InsertOne(context.Background(), newB)
	}

	if _, err := r.Update(context.Background(), newB, oldB); err != nil {
		return fmt.Errorf("updating record: %w", err)
	}

	if err := gitUpdate(r.Cfg.Fullpath(), oldB, newB); err != nil {
		return err
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", newB.ID)))

	return nil
}

// removeRecords removes the records from the database.
func removeRecords(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) error {
	sp := rotato.New(
		rotato.WithMesg("removing record/s..."),
		rotato.WithMesgColor(rotato.ColorGray),
	)
	sp.Start()
	defer sp.Done()

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

	if err := gitClean(r.Cfg.Fullpath(), bs); err != nil {
		return err
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("%d bookmark/s removed\n", len(bs))))

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
func byTags(r *db.SQLite, tags []string, bs *slice.Slice[bookmark.Bookmark]) error {
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
func byQuery(r *db.SQLite, bs *slice.Slice[bookmark.Bookmark], args []string) error {
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
func byIDs(r *db.SQLite, bs *slice.Slice[bookmark.Bookmark], args []string) error {
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
	var (
		rawArgs = os.Args[1:]
		order   = []string{}
	)

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
