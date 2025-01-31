package handler

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
)

var (
	ErrInvalidOption = errors.New("invalid option")
	ErrNoItems       = errors.New("no items")
)

// ByTags returns a slice of bookmarks based on the provided tags.
func ByTags(r *repo.SQLiteRepository, tags []string, bs *Slice) error {
	// FIX: redo, simplify
	// if the slice contains bookmarks, filter by tag.
	if !bs.Empty() {
		for _, tag := range tags {
			bs.Filter(func(b Bookmark) bool {
				return strings.Contains(b.Tags, tag)
			})
		}

		return nil
	}

	for _, tag := range tags {
		if err := r.ByTag(tag, bs); err != nil {
			return fmt.Errorf("byTags :%w", err)
		}
	}

	if bs.Empty() {
		t := strings.Join(tags, ", ")
		return fmt.Errorf("%w by tag: '%s'", repo.ErrRecordNoMatch, t)
	}

	bs.Filter(func(b Bookmark) bool {
		for _, tag := range tags {
			if !strings.Contains(b.Tags, tag) {
				return false
			}
		}

		return true
	})

	return nil
}

// ByQuery executes a search query on the given repository based on provided
// arguments.
func ByQuery(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	if bs.Len() != 0 || len(args) == 0 {
		return nil
	}

	q := strings.Join(args, "%")
	if err := r.ByQuery(r.Cfg.Tables.Main, q, bs); err != nil {
		return fmt.Errorf("%w: '%s'", err, strings.Join(args, " "))
	}

	return nil
}

// ByIds retrieves records from the database based on either
// an ID or a query string.
func ByIDs(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	ids, err := extractIDsFrom(args)
	if len(ids) == 0 {
		return nil
	}

	if !errors.Is(err, bookmark.ErrInvalidID) && err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := r.ByIDList(r.Cfg.Tables.Main, ids, bs); err != nil {
		return fmt.Errorf("records from args: %w", err)
	}

	if bs.Empty() {
		bids := strings.TrimRight(strings.Join(args, ", "), "\n")
		return fmt.Errorf("%w by id/s: %s", repo.ErrRecordNotFound, bids)
	}

	return nil
}

// ByHeadAndTail returns a slice of bookmarks with limited elements.
func ByHeadAndTail(bs *Slice, h, t int) error {
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

// Selection allows the user to select multiple records in a menu
// interface.
func Selection[T comparable](m *menu.Menu[T], items *[]T, fmtFn func(*T) string) ([]T, error) {
	if len(*items) == 0 {
		return nil, repo.ErrRecordNoMatch
	}

	var result []T
	result, err := m.Select(items, fmtFn)
	if err != nil {
		if errors.Is(err, menu.ErrFzfActionAborted) {
			return nil, ErrActionAborted
		}

		return nil, fmt.Errorf("%w", err)
	}

	if len(result) == 0 {
		return nil, ErrNoItems
	}

	return result, nil
}

// MenuDefaults returns the options for the menu.
func MenuDefaults(multiline bool) []menu.OptFn {
	// menu opts
	opts := []menu.OptFn{
		menu.WithDefaultKeybinds(),
		menu.WithDefaultSettings(),
		menu.WithMultiSelection(),
	}

	if !subCommandCalled {
		opts = append(opts,
			menu.WithPreview(),
			menu.WithKeybindEdit(),
			menu.WithKeybindOpen(),
			menu.WithKeybindQR(),
			menu.WithKeybindOpenQR(),
		)
	}

	if multiline {
		opts = append(opts, menu.WithMultilineView())
	}

	return opts
}
