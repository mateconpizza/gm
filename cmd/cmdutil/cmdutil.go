package cmdutil

import (
	"context"
	"fmt"

	menu "github.com/mateconpizza/go-fzf"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type (
	// BookmarkAction defines a task to be performed on a set of bookmarks.
	BookmarkAction func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error

	// Filter narrows down a slice of bookmarks.
	Filter func([]*bookmark.Bookmark) []*bookmark.Bookmark

	// RunFunc is a command action that only requires dependency setup.
	RunFunc func(ctx context.Context, d *deps.Deps) error
)

// setup initializes the application dependencies for subcommands.
func setup(ctx context.Context, args *[]string) (*deps.Deps, func(), error) {
	app, err := application.FromContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return nil, nil, err
	}

	terminal.ReadPipedInput(args)

	c := ui.NewDefaultConsole(app, func(err error) {
		r.Close()
		app.Exit(err)
	})

	d := deps.New(
		deps.WithApplication(app),
		deps.WithRepo(r),
		deps.WithConsole(c),
	)

	return d, r.Close, nil
}

func Execute(ctx context.Context, args []string, m *menu.Menu[*bookmark.Bookmark], action BookmarkAction, filters ...Filter) error {
	return Run(ctx, args, func(ctx context.Context, d *deps.Deps) error {
		bs, err := handler.Data(ctx, d, args)
		if err != nil {
			return err
		}

		app, err := d.Application(ctx)
		if err != nil {
			return err
		}

		f := app.Flags

		bs, err = sortBookmarks(bs, f.Sort, f.Reverse)
		if err != nil {
			return err
		}

		bs, err = filterBookmarks(bs, f.Filter, filters, f.Head, f.Tail)
		if err != nil {
			return err
		}

		if f.Menu && len(bs) > 0 {
			bs, err = picker.BookmarkWithMenu(ctx, d.Console(), m, bs)
			if err != nil {
				return err
			}
		}

		return action(ctx, d, bs)
	})
}

// sortBookmarks applies the requested sort order and optional reversal.
func sortBookmarks(bs []*bookmark.Bookmark, order string, reverse bool) ([]*bookmark.Bookmark, error) {
	var err error

	bs, err = bookmark.Sort(order, bs)
	if err != nil {
		return nil, err
	}

	if reverse {
		bs, _ = bookmark.Sort("reverse", bs)
	}

	return bs, nil
}

// filterBookmarks applies configured, custom, and head/tail filters in order.
func filterBookmarks(bs []*bookmark.Bookmark, filter string, filters []Filter, head, tail int) ([]*bookmark.Bookmark, error) {
	var err error

	bs, err = bookmark.FilterBy(filter, bs)
	if err != nil {
		return nil, err
	}

	for _, apply := range filters {
		bs = apply(bs)
	}

	if head > 0 || tail > 0 {
		bs, err = handler.FilterByHeadAndTail(bs, head, tail)
		if err != nil {
			return nil, fmt.Errorf("failed to filter by head/tail: %w", err)
		}
	}

	return bs, nil
}

// Run sets up command dependencies, runs the action, and handles cleanup.
func Run(ctx context.Context, args []string, fn RunFunc) error {
	d, cleanup, err := setup(ctx, &args)
	if err != nil {
		return err
	}

	defer cleanup()

	return fn(ctx, d)
}
