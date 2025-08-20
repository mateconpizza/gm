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

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrInvalidOption = errors.New("invalid option")
	ErrNoItems       = errors.New("no items")
)

// Data retrieves and filters bookmarks based on configuration and arguments.
func Data(
	m *menu.Menu[bookmark.Bookmark],
	r *db.SQLite,
	args []string,
) ([]*bookmark.Bookmark, error) {
	f := config.App.Flags

	// Get initial records
	bs, err := getRecords(r, args)
	if err != nil {
		return nil, fmt.Errorf("failed to get records: %w", err)
	}

	// Apply filters
	bs, err = applyFilters(r, bs, f)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters: %w", err)
	}

	if len(bs) == 0 {
		return nil, ErrNoItems
	}

	// Apply menu selection if needed
	if f.Menu || f.Multiline {
		bs, err = applyMenuSelection(m, bs, f)
		if err != nil {
			return nil, fmt.Errorf("failed to apply menu selection: %w", err)
		}
	}

	return bs, nil
}

// getRecords retrieves records based on user input and filtering criteria.
func getRecords(r *db.SQLite, args []string) ([]*bookmark.Bookmark, error) {
	slog.Debug("getRecords", "args", args)

	// Try to get by IDs first
	if bs, err := getByIDs(r, args); err != nil {
		// If it's not an invalid ID error, return the error
		if !errors.Is(err, bookmark.ErrBookmarkInvalidID) {
			return nil, err
		}
		// If invalid ID, try query instead
	} else if len(bs) > 0 {
		return bs, nil
	}

	// Try to get by query
	if bs, err := getByQuery(r, args); err != nil {
		return nil, err
	} else if len(bs) > 0 {
		return bs, nil
	}

	// If no args provided or nothing found, get all records
	if len(args) == 0 {
		return r.All(context.Background())
	}

	// No results found
	return []*bookmark.Bookmark{}, nil
}

// getByIDs retrieves records from the database based on IDs.
func getByIDs(r *db.SQLite, args []string) ([]*bookmark.Bookmark, error) {
	slog.Debug("getting by IDs")

	ids, err := extractIDsFrom(args)
	if len(ids) == 0 {
		return nil, bookmark.ErrBookmarkInvalidID // Signal that this isn't the right method
	}

	if err != nil && !errors.Is(err, bookmark.ErrBookmarkInvalidID) {
		return nil, fmt.Errorf("failed to extract IDs: %w", err)
	}

	bs, err := r.ByIDList(context.Background(), ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get records by ID list: %w", err)
	}

	if len(bs) == 0 {
		bids := strings.TrimRight(strings.Join(args, ", "), "\n")
		return nil, fmt.Errorf("%w by id/s: %s in %q", db.ErrRecordNotFound, bids, r.Name())
	}

	return bs, nil
}

// getByQuery executes a search query on the given repository.
func getByQuery(r *db.SQLite, args []string) ([]*bookmark.Bookmark, error) {
	if len(args) == 0 {
		return []*bookmark.Bookmark{}, nil
	}

	q := strings.Join(args, "%")
	bs, err := r.ByQuery(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("query failed for %q: %w", strings.Join(args, " "), err)
	}

	return bs, nil
}

// applyFilters applies tag and head/tail filters to the bookmark list.
func applyFilters(r *db.SQLite, bs []*bookmark.Bookmark, f *config.Flags) ([]*bookmark.Bookmark, error) {
	var err error

	// Filter by tags
	if len(f.Tags) > 0 {
		bs, err = filterByTags(r, f.Tags, bs)
		if err != nil {
			return nil, fmt.Errorf("failed to filter by tags: %w", err)
		}
	}

	// Filter by head and tail
	if f.Head > 0 || f.Tail > 0 {
		bs, err = filterByHeadAndTail(bs, f.Head, f.Tail)
		if err != nil {
			return nil, fmt.Errorf("failed to filter by head/tail: %w", err)
		}
	}

	return bs, nil
}

// filterByTags returns a slice of bookmarks that contain ALL of the provided tags.
func filterByTags(r *db.SQLite, tags []string, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	n := len(bs)
	slog.Debug("by tags", "tags", tags, "count", n)

	// If we have existing bookmarks, filter them by tags
	if n > 0 {
		return filterBookmarksByTags(bs, tags), nil
	}

	// Otherwise, get bookmarks from repository by tags
	// We need to find the intersection of bookmarks that have ALL tags
	if len(tags) == 0 {
		return []*bookmark.Bookmark{}, nil
	}

	// Get bookmarks for the first tag
	firstTag := ""
	for _, tag := range tags {
		if tag != "" {
			firstTag = tag
			break
		}
	}

	if firstTag == "" {
		return []*bookmark.Bookmark{}, nil
	}

	candidates, err := r.ByTag(context.Background(), firstTag)
	if err != nil {
		return nil, fmt.Errorf("failed to get bookmarks by tag %q: %w", firstTag, err)
	}

	// Filter candidates to only include those that have ALL remaining tags
	remainingTags := make([]string, 0, len(tags)-1)
	for _, tag := range tags {
		if tag != "" && tag != firstTag {
			remainingTags = append(remainingTags, tag)
		}
	}

	result := filterBookmarksByTags(candidates, append([]string{firstTag}, remainingTags...))

	if len(result) == 0 {
		tagsList := strings.Join(tags, ", ")
		return nil, fmt.Errorf("%w by tags: %q", db.ErrRecordNoMatch, tagsList)
	}

	return result, nil
}

// filterBookmarksByTags filters existing bookmarks by tags.
func filterBookmarksByTags(bs []*bookmark.Bookmark, tags []string) []*bookmark.Bookmark {
	if len(tags) == 0 {
		return bs
	}

	var result []*bookmark.Bookmark
	for _, b := range bs {
		hasAllTags := true

		// Check if bookmark has ALL required tags
		for _, tag := range tags {
			if tag != "" && !strings.Contains(b.Tags, tag) {
				hasAllTags = false
				break
			}
		}

		if hasAllTags {
			result = append(result, b)
		}
	}
	return result
}

// filterByHeadAndTail returns a slice of bookmarks with limited elements based
// on head/tail parameters.
func filterByHeadAndTail(bs []*bookmark.Bookmark, h, t int) ([]*bookmark.Bookmark, error) {
	if h == 0 && t == 0 {
		return bs, nil
	}

	if h < 0 || t < 0 {
		return nil, fmt.Errorf("%w: head=%d tail=%d", ErrInvalidOption, h, t)
	}

	if len(bs) == 0 {
		return bs, nil
	}

	// Determine flag order from command line args
	order := getFlagOrder()
	result := make([]*bookmark.Bookmark, len(bs))
	copy(result, bs)

	for _, action := range order {
		switch action {
		case "head":
			if h > 0 {
				result = head(result, h)
			}
		case "tail":
			if t > 0 {
				result = tail(result, t)
			}
		}
	}

	return result, nil
}

// getFlagOrder determines the order of head/tail flags from command line.
func getFlagOrder() []string {
	rawArgs := os.Args[1:]
	var order []string

	for _, arg := range rawArgs {
		if strings.HasPrefix(arg, "-H") || strings.HasPrefix(arg, "--head") {
			order = append(order, "head")
		} else if strings.HasPrefix(arg, "-T") || strings.HasPrefix(arg, "--tail") {
			order = append(order, "tail")
		}
	}

	return order
}

// head returns the first n elements of the slice.
func head(bs []*bookmark.Bookmark, n int) []*bookmark.Bookmark {
	if n <= 0 || len(bs) == 0 {
		return []*bookmark.Bookmark{}
	}

	if n >= len(bs) {
		return bs
	}

	result := make([]*bookmark.Bookmark, n)
	copy(result, bs[:n])
	return result
}

// tail returns the last n elements of the slice.
func tail(bs []*bookmark.Bookmark, n int) []*bookmark.Bookmark {
	l := len(bs)
	if n <= 0 || l == 0 {
		return []*bookmark.Bookmark{}
	}

	if n >= l {
		return bs
	}

	start := l - n
	result := make([]*bookmark.Bookmark, n)
	copy(result, bs[start:])
	return result
}

// applyMenuSelection applies menu selection to bookmarks.
func applyMenuSelection(
	m *menu.Menu[bookmark.Bookmark],
	bs []*bookmark.Bookmark,
	f *config.Flags,
) ([]*bookmark.Bookmark, error) {
	// Create copy for menu selection
	bsCopy := make([]bookmark.Bookmark, 0, len(bs))
	for _, b := range bs {
		bsCopy = append(bsCopy, *b)
	}

	// Select with menu
	items, err := selectionWithMenu(m, bsCopy, func(b *bookmark.Bookmark) string {
		if f.Multiline {
			return txt.Multiline(b)
		}
		return txt.Oneline(b)
	})
	if err != nil {
		return nil, fmt.Errorf("menu selection failed: %w", err)
	}

	// Convert selected items back to pointers
	result := make([]*bookmark.Bookmark, len(items))
	for i := range items {
		result[i] = &items[i]
	}

	return result, nil
}

func handleEditedBookmark(c *ui.Console, r *db.SQLite, newB, oldB *bookmark.Bookmark) error {
	// is a new bookmark
	newBookmark := newB.ID == 0
	if newBookmark {
		_, err := r.InsertOne(context.Background(), newB)
		if err != nil {
			return err
		}

		return nil
	}

	if err := r.UpdateOne(context.Background(), newB); err != nil {
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

	if err := r.DeleteMany(context.Background(), bs); err != nil {
		return err
	}

	sp.Done()

	if err := gitClean(r.Cfg.Fullpath(), bs); err != nil {
		return err
	}

	if c != nil {
		fmt.Print(c.SuccessMesg(fmt.Sprintf("%d bookmark/s removed\n", len(bs))))
	}

	return nil
}

// FindDB returns the path to the database.
func FindDB(p string) (string, error) {
	slog.Debug("searching db", "path", p)

	if files.Exists(p) {
		return p, nil
	}

	fs, err := files.FindByExtList(filepath.Dir(p), ".db", ".enc")
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
