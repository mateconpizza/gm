package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/repo"
)

var (
	ErrInvalidOption = errors.New("invalid option")
	ErrNoItems       = errors.New("no items")
)

// ByTags returns a slice of bookmarks based on the provided tags.
func ByTags(r *repo.SQLiteRepository, tags []string, bs *Slice) error {
	slog.Debug("by tags", "tags", tags, "count", bs.Len())
	// FIX: redo, simplify
	// if the slice contains bookmarks, filter by tag.
	if !bs.Empty() {
		for _, tag := range tags {
			bs.FilterInPlace(func(b *Bookmark) bool {
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
		return fmt.Errorf("%w by tag: %q", repo.ErrRecordNoMatch, t)
	}

	bs.FilterInPlace(func(b *Bookmark) bool {
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

// ByIDs retrieves records from the database based on either
// an ID or a query string.
func ByIDs(r *repo.SQLiteRepository, bs *Slice, args []string) error {
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
		return fmt.Errorf("%w by id/s: %s in %q", repo.ErrRecordNotFound, bids, r.Name())
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
