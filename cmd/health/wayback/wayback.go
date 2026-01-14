// Package wayback provides commands for querying the Internet Archive Wayback
// Machine to retrieve historical snapshots of bookmarked URLs.
package wayback

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
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

func NewCmd(cfg *config.Config) *cobra.Command {
	waybackCmd := &cobra.Command{
		Use:     "wayback",
		Aliases: []string{"w", "wm", "wb"},
		Short:   "Query the Wayback Machine for bookmarks",
		Long:    `Query the Internet Archive Wayback Machine for one or more bookmarks.`,
		Example: `  # Get the latest snapshot for bookmark 179
  gm health wayback --latest 179

  # Get up to 5 snapshots from 2023
  gm health wayback --limit 5 --year 2023 179`,
		RunE: waybackFunc,
	}

	f := waybackCmd.Flags()
	f.SortFlags = false
	f.BoolVarP(&cfg.Flags.Snapshot, "latest", "l", false, "fetches lastets snapshot from Wayback Machine")
	f.IntVarP(&cfg.Flags.Limit, "limit", "L", 0, "limit the number of snapshots returned")
	f.IntVarP(&cfg.Flags.Year, "year", "Y", 0, "fetches the last N snapshots from a specific year")
	f.BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "interactive menu mode using fzf (select bookmarks)")
	f.BoolVar(&cfg.Flags.Multiline, "multiline", false, "output in multiline format (fzf)")

	return waybackCmd
}

func waybackFunc(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)
	a := app.New(cmd.Context(),
		app.WithDB(r),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	m := handler.MenuSimple[bookmark.Bookmark](cfg, menu.WithMultiSelection(),
		menu.WithHeader("select record/s"))

	return processWayback(a, m, args)
}

func processWayback(a *app.Context, m *menu.Menu[bookmark.Bookmark], args []string) error {
	bs, err := handler.Data(a, m, args)
	if err != nil {
		return err
	}

	n := len(bs)
	if n == 0 {
		return db.ErrRecordNotFound
	}

	flags := a.Cfg.Flags
	if n > wayback.MaxItems && !flags.Force {
		return wayback.ErrTooManyRecords
	}

	operation := "all available snapshots"
	if flags.Snapshot {
		operation = "latest snapshot"
	} else if flags.Limit > 0 {
		operation = fmt.Sprintf("up to %d snapshot(s)", flags.Limit)
	}

	if !flags.Snapshot && flags.Year > 0 {
		operation += fmt.Sprintf(" from %d", flags.Year)
	}

	f, p := a.Console().Frame(), a.Console().Palette()
	f.Headerln("Wayback Machine: Fetch " + operation).Rowln()
	f.Midln(p.BrightCyan.Sprintf("Selected bookmarks: %d", n)).Rowln()
	for i := range bs {
		if i >= wayback.MaxItems {
			f.Midln(p.BrightBlack.With(p.Italic).Sprintf("... and %d more", n-i))
			break
		}
		f.Midln(p.BrightBlack.Sprintf("[%d] ", bs[i].ID) + bs[i].URL)
	}

	f.Rowln().Flush()

	if !a.Console().Confirm("continue with Wayback Machine query?", "n") {
		return sys.ErrExitFailure
	}

	if flags.Snapshot {
		return handler.WaybackLatestSnapshot(a, bs)
	}

	return handler.WaybackSnapshots(a, bs)
}
