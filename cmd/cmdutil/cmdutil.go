package cmdutil

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type (
	// BookmarkAction defines a task to be performed on a set of bookmarks.
	BookmarkAction func(*deps.Deps, []*bookmark.Bookmark) error

	// Filter is a predicate used to narrow down a slice of bookmarks
	// before they are passed to an action or presented in a menu.
	Filter func([]*bookmark.Bookmark) []*bookmark.Bookmark
)

// SetupDeps initializes the config, db and app for the subcommands..
func SetupDeps(cmd *cobra.Command, args *[]string) (*deps.Deps, func(), error) {
	ctx := cmd.Context()

	app, err := application.FromContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(ctx, app.Path.Database)
	if err != nil {
		return nil, nil, err
	}

	terminal.ReadPipedInput(args)

	console := ui.NewDefaultConsole(ctx, func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	})

	d := deps.New(
		ctx,
		deps.WithApplication(app),
		deps.WithRepo(r),
		deps.WithConsole(console),
	)

	return d, r.Close, nil
}

func Execute(
	cmd *cobra.Command,
	args []string,
	m *menu.Menu[bookmark.Bookmark],
	action BookmarkAction,
	filters ...Filter,
) error {
	d, cleanup, err := SetupDeps(cmd, &args)
	if err != nil {
		return err
	}
	defer cleanup()

	bs, err := handler.Data(d, args)
	if err != nil {
		return err
	}

	app, err := d.Application()
	if err != nil {
		return err
	}

	// sort items
	f := app.Flags
	bs, err = handler.Sort(f.Sort, bs)
	if err != nil {
		return err
	}

	// custom filters
	for _, filter := range filters {
		bs = filter(bs)
	}

	// filter by head and tail
	if f.Head > 0 || f.Tail > 0 {
		bs, err = handler.FilterByHeadAndTail(bs, f.Head, f.Tail)
		if err != nil {
			return fmt.Errorf("failed to filter by head/tail: %w", err)
		}
	}

	// menu selection
	if f.Menu && len(bs) > 0 {
		bs, err = picker.BookmarkWithMenu(m, bs)
		if err != nil {
			return err
		}
	}

	return action(d, bs)
}
