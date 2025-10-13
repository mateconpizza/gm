package wayback

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/scraper/wayback"
)

func NewCmd() *cobra.Command {
	app := config.New()
	waybackCmd := &cobra.Command{
		Use:     "wayback",
		Aliases: []string{"w", "wm", "wb"},
		Short:   "Wayback Machine",
		RunE:    waybackFunc,
	}

	f := waybackCmd.Flags()
	f.SortFlags = false
	f.BoolVarP(&app.Flags.Snapshot, "latest", "c", false,
		"fetches lastets snapshot from Wayback Machine")
	f.IntVarP(&app.Flags.Limit, "limit", "L", 0,
		"limit the number of snapshots returned")
	f.IntVarP(&app.Flags.Year, "year", "Y", 0,
		"fetches the last N snapshots from a specific year")
	f.BoolVarP(&app.Flags.Menu, "menu", "m", false,
		"interactive menu mode using fzf (select bookmarks)")

	return waybackCmd
}

func waybackFunc(command *cobra.Command, args []string) error {
	app := config.New()
	r, err := db.New(app.DBPath)
	if err != nil {
		return err
	}

	terminal.ReadPipedInput(&args)

	defer r.Close()
	bs, err := handler.Data(menuForRecords(app), r, args, app.Flags)
	if err != nil {
		return err
	}

	n := len(bs)
	if n == 0 {
		return db.ErrRecordNotFound
	}

	f := app.Flags
	if n > wayback.MaxItems && !f.Force {
		return fmt.Errorf("%w: %d", wayback.ErrTooManyRecords, n)
	}

	c := ui.NewDefaultConsole(command.Context(), func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	})

	switch {
	case f.Snapshot:
		return handler.WaybackLatestSnapshot(c, r, bs)
	case f.Limit > 0:
		return handler.WaybackSnapshots(c, r, bs)
	}

	return command.Help()
}

func menuForRecords[T bookmark.Bookmark](app *config.Config) *menu.Menu[T] {
	return menu.New[T](
		menu.WithSettings(app.Menu.Settings),
		menu.WithPreview(app.Cmd+" --name "+app.DBName+" records {1}"),
	)
}
