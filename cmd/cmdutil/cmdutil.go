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

	// Filter is a predicate used to narrow down a slice of bookmarks
	// before they are passed to an action or presented in a menu.
	Filter func([]*bookmark.Bookmark) []*bookmark.Bookmark

	// RunFunc is a command action that only requires dependency setup.
	RunFunc func(ctx context.Context, d *deps.Deps) error
)

// setup initializes the config, db and app for the subcommands..
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

func Bookmarks(ctx context.Context, app *application.App, fn func(bs []*bookmark.Bookmark) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	bs, err := r.All(ctx)
	if err != nil {
		return err
	}

	if app.Flags.Menu {
		m := picker.NewWithFormatter(app, app.Formatter(), menu.WithMultiSelection())
		bs, err = m.Select(ctx, bs)
		if err != nil {
			return err
		}
	}

	return fn(bs)
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
		bs, err = bookmark.Sort(f.Sort, bs)
		if err != nil {
			return err
		}

		if f.Reverse {
			bs, _ = bookmark.Sort("reverse", bs)
		}

		bs, err = bookmark.FilterBy(f.Filter, bs)
		if err != nil {
			return err
		}

		for _, filter := range filters {
			bs = filter(bs)
		}

		if f.Head > 0 || f.Tail > 0 {
			bs, err = handler.FilterByHeadAndTail(bs, f.Head, f.Tail)
			if err != nil {
				return fmt.Errorf("failed to filter by head/tail: %w", err)
			}
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

// Run sets up command dependencies, runs action, and handles cleanup.
func Run(ctx context.Context, args []string, fn RunFunc) error {
	d, cleanup, err := setup(ctx, &args)
	if err != nil {
		return err
	}
	defer cleanup()

	return fn(ctx, d)
}
