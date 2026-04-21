// Package archive provides commands for querying the Internet Archive Wayback
// Machine to retrieve historical snapshots of bookmarked URLs.
package archive

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper/wayback"
)

func newLookupCmd(app *application.App) *cobra.Command {
	use := "fetch"
	c := &cobra.Command{
		Use:   use,
		Short: "wayback lookup",
		Example: fmt.Sprintf(`  # get the latest snapshot for bookmark 179
  %s archive %s 179 --latest

  # get up to 5 snapshots from 2023
  %s archive %s 179 --limit 5 --year 2023 179`, app.Cmd, use, app.Cmd, use),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" wayback machine lookup "),
				menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				op := waybackOperation(app.Flags)
				if !confirmWayback(d, bs, op) {
					return sys.ErrExitFailure
				}

				return runWayback(d, app.Flags, bs)
			})
		},
	}

	f := c.Flags()
	f.SortFlags = false
	cmdutil.FlagMenu(c, app)
	f.BoolVarP(&app.Flags.Update, "latest", "l", false, "fetches lasts snapshot from Wayback Machine")
	f.IntVarP(&app.Flags.Limit, "limit", "L", 0, "limit the number of snapshots returned")
	f.IntVarP(&app.Flags.Year, "year", "Y", 0, "fetches the last N snapshots from a specific year")
	cmdutil.FlagsFilter(c, app)
	cmdutil.HideFlag(c, "help")

	return c
}

func runWayback(d *deps.Deps, flags *application.Flags, bs []*bookmark.Bookmark) error {
	if flags.Update {
		return handler.WaybackLatestSnapshot(d, bs)
	}
	return handler.WaybackSnapshots(d, bs)
}

func confirmWayback(d *deps.Deps, bs []*bookmark.Bookmark, op string) bool {
	f, p := d.Console().Frame(), d.Console().Palette()

	f.Headerln("Wayback Machine: Fetch " + op).Rowln()
	f.Midln(p.BrightCyan.Sprintf("[%d] selected bookmarks:", len(bs))).Rowln()

	for i := range bs {
		if i >= wayback.MaxItems {
			f.Midln(p.BrightBlack.With(p.Italic).
				Sprintf("... and %d more", len(bs)-i))
			break
		}
		f.Midln(p.BrightBlack.Sprintf("[%d] ", bs[i].ID) + bs[i].URL)
	}

	f.Rowln().Flush()

	return d.Console().Confirm("continue with Wayback Machine query?", "n")
}

func waybackOperation(f *application.Flags) string {
	op := "all available snapshots"

	if f.Update {
		return "latest snapshot"
	}
	if f.Limit > 0 {
		op = fmt.Sprintf("up to %d snapshot(s)", f.Limit)
	}
	if f.Year > 0 {
		op += fmt.Sprintf(" from %d", f.Year)
	}

	return op
}
