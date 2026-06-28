// Package archive provides commands for querying the Internet Archive Wayback
// Machine to retrieve historical snapshots of bookmarked URLs.
package archive

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper/wayback"
)

func newLookupCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "fetch",
		Short: "wayback lookup",
		Example: app.Example(`  $ {cmd} url archive fetch 179 --latest
  $ {cmd} url archive fetch 179 --limit 5
  $ {cmd} url archive fetch 179 --limit 5 --year 2023
  $ {cmd} url archive fetch --menu
  $ {cmd} url archive fetch 179 --duration 10s`),
		RunE: func(cmd *cobra.Command, args []string) error {
			fm := app.MenuFormatter()
			p := fm.Menu.Placeholder()
			m := picker.NewWithFormatter(
				app,
				fm,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" wayback machine lookup "),
				menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), p.Single())),
			)

			a := func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				op := waybackOperation(app.Flags)
				if !confirmWayback(cmd.Context(), d, bs, op) {
					return sys.ErrExitFailure
				}
				return runWayback(ctx, d, app.Flags, bs)
			}

			return cmdutil.Execute(cmd, args, m, a)
		},
	}

	f := c.Flags()
	f.SortFlags = false
	f.BoolVarP(&app.Flags.Update, "latest", "l", false, "fetches lasts snapshot from Wayback Machine")
	f.IntVarP(&app.Flags.Limit, "limit", "L", 0, "return at most N snapshots")
	f.IntVarP(&app.Flags.Year, "year", "Y", 0, "restrict snapshots to a specific year")
	f.DurationVar(&app.Flags.Duration, "duration", 30*time.Second, "maximum time to wait for snapshot retrieval")

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}

func runWayback(ctx context.Context, d *deps.Deps, flags *application.Flags, bs []*bookmark.Bookmark) error {
	if flags.Update {
		return handler.WaybackLatestSnapshot(ctx, d, bs)
	}
	return handler.WaybackSnapshots(ctx, d, bs)
}

func confirmWayback(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark, op string) bool {
	f, p := d.Console().Frame(), d.Console().Palette()

	title := p.BrightYellow.
		Wrap("Wayback Machine: Fetch "+op, p.Bold)

	subtitle := p.Dim.With(p.Italic).
		Sprint("confirm bookmarks to query in the wayback machine")

	items := p.BrightCyan.
		Sprintf("[%d] selected bookmarks:", len(bs))
	header := func() string {
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}
	selected := func() string {
		return p.BrightCyan.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	f.CustomFunc(header, title).Ln().
		Headerln(subtitle).
		Rowln().
		CustomFunc(selected, items).Ln().
		Rowln()

	for i := range bs {
		if i >= wayback.MaxItems {
			f.Midln(p.Gray.With(p.Italic).
				Sprintf("... and %d more", len(bs)-i))
			break
		}
		f.Midln(p.Gray.Sprintf("[%d] ", bs[i].ID) + bs[i].URL)
	}

	f.Rowln().Flush()

	return d.Console().Confirm(ctx, "continue?", "n")
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
