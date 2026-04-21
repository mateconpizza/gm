package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var (
	ErrInvalidOption = errors.New("invalid option")
	ErrNoItems       = errors.New("no items")
)

// Data retrieves and filters bookmarks based on configuration and arguments.
func Data(d *deps.Deps, args []string) ([]*bookmark.Bookmark, error) {
	bs, err := fetchBookmarks(d, args)
	if err != nil {
		return nil, err
	}

	bs, err = applyFilters(d, bs)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters: %w", err)
	}

	if len(bs) == 0 {
		return nil, ErrNoItems
	}

	return bs, nil
}

// fetchBookmarks retrieves records based on user input and filtering criteria.
func fetchBookmarks(d *deps.Deps, args []string) ([]*bookmark.Bookmark, error) {
	slog.Debug("FetchBookmarks", "args", args)

	// Try to get by IDs first
	if bs, err := getByIDs(d, args); err != nil {
		if !errors.Is(err, bookmark.ErrBookmarkInvalidID) {
			return nil, err
		}
		// If invalid ID, try query instead
	} else if len(bs) > 0 {
		return bs, nil
	}

	// Try to get by query
	if bs, err := getByQuery(d, args); err != nil {
		return nil, err
	} else if len(bs) > 0 {
		return bs, nil
	}

	// If no args provided or nothing found, get all records
	if len(args) == 0 {
		return d.Repo.All(d.Context())
	}

	// No results found
	return []*bookmark.Bookmark{}, nil
}

// getByIDs retrieves records from the database based on IDs.
func getByIDs(d *deps.Deps, args []string) ([]*bookmark.Bookmark, error) {
	slog.Debug("getting by IDs")

	ids, err := extractIDsFrom(args)
	if len(ids) == 0 {
		return nil, bookmark.ErrBookmarkInvalidID // Signal that this isn't the right method
	}

	if err != nil && !errors.Is(err, bookmark.ErrBookmarkInvalidID) {
		return nil, fmt.Errorf("failed to extract IDs: %w", err)
	}

	bs, err := d.Repo.ByIDList(d.Context(), ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get records by ID list: %w", err)
	}

	if len(bs) == 0 {
		bids := strings.TrimRight(strings.Join(args, ", "), "\n")
		return nil, fmt.Errorf("%w by id/s: %s in %q", db.ErrRecordNotFound, bids, d.Repo.Name())
	}

	return bs, nil
}

// getByQuery executes a search query on the given repository.
func getByQuery(d *deps.Deps, args []string) ([]*bookmark.Bookmark, error) {
	if len(args) == 0 {
		return []*bookmark.Bookmark{}, nil
	}

	q := strings.Join(args, "%")
	bs, err := d.Repo.ByQuery(d.Context(), q)
	if err != nil {
		return nil, fmt.Errorf("%w %q", err, strings.Join(args, " "))
	}

	return bs, nil
}

// applyFilters applies tag and head/tail filters to the bookmark list.
func applyFilters(d *deps.Deps, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	var err error
	f := d.App.Flags

	// Filter by tags
	if len(f.Tags) > 0 {
		bs, err = filterByTags(d, f.Tags, bs)
		if err != nil {
			return nil, fmt.Errorf("failed to filter by tags: %w", err)
		}
	}

	return bs, nil
}

// filterByTags returns a slice of bookmarks that contain ALL of the provided tags.
func filterByTags(d *deps.Deps, tags []string, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
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

	candidates, err := d.Repo.ByTag(d.Context(), firstTag)
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

// FilterByHeadAndTail returns a slice of bookmarks with limited elements based
// on head/tail parameters.
func FilterByHeadAndTail(bs []*bookmark.Bookmark, h, t int) ([]*bookmark.Bookmark, error) {
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

// removeRecords removes the records from the database.
func removeRecords(d *deps.Deps, bs []*bookmark.Bookmark) error {
	sp := rotato.New(
		rotato.WithMesg("removing record/s..."),
		rotato.WithMesgColor(rotato.ColorGray),
	)
	sp.Start()
	defer sp.Done()

	if err := d.Repo.DeleteMany(d.Context(), bs); err != nil {
		return err
	}

	sp.Done()

	if err := git.RemoveBookmarks(d.App, bs); err != nil {
		return err
	}

	if d.Console() != nil {
		fmt.Print(d.Console().SuccessMesg(fmt.Sprintf("%d bookmark/s removed\n", len(bs))))
	}

	return nil
}
