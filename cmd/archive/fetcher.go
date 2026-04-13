// Package archive provides commands for querying the Internet Archive Wayback
// Machine to retrieve historical snapshots of bookmarked URLs.
package archive

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/base"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper/wayback"
)

func newLookupCmd(cfg *config.Config) *cobra.Command {
	use := "fetch"
	c := &cobra.Command{
		Use:   use,
		Short: "wayback lookup",
		Example: fmt.Sprintf(`  # get the latest snapshot for bookmark 179
  %s archive %s --latest 179

  # get up to 5 snapshots from 2023
  %s archive %s --limit 5 --year 2023 179`, cfg.Cmd, use, cfg.Cmd, use),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cfg.Flags
			m := handler.MenuSimple[bookmark.Bookmark](
				cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" wayback machine lookup "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return base.Execute(cmd, args, m, func(a *app.Context, bs []*bookmark.Bookmark) error {
				operation := "all available snapshots"
				if flags.Update {
					operation = "latest snapshot"
				} else if flags.Limit > 0 {
					operation = fmt.Sprintf("up to %d snapshot(s)", flags.Limit)
				}

				if !flags.Update && flags.Year > 0 {
					operation += fmt.Sprintf(" from %d", flags.Year)
				}

				f, p := a.Console().Frame(), a.Console().Palette()
				f.Headerln("Wayback Machine: Fetch " + operation).Rowln()
				f.Midln(p.BrightCyan.Sprintf("[%d] selected bookmarks:", len(bs))).Rowln()
				for i := range bs {
					if i >= wayback.MaxItems {
						f.Midln(p.BrightBlack.With(p.Italic).Sprintf("... and %d more", len(bs)-i))
						break
					}
					f.Midln(p.BrightBlack.Sprintf("[%d] ", bs[i].ID) + bs[i].URL)
				}

				f.Rowln().Flush()
				if !a.Console().Confirm("continue with Wayback Machine query?", "n") {
					return sys.ErrExitFailure
				}
				if flags.Update {
					return handler.WaybackLatestSnapshot(a, bs)
				}
				return handler.WaybackSnapshots(a, bs)
			})
		},
	}

	f := c.Flags()
	f.SortFlags = false
	base.FlagMenu(c, cfg)
	base.FlagsFilter(c, cfg)
	f.BoolVarP(&cfg.Flags.Update, "latest", "l", false, "fetches lasts snapshot from Wayback Machine")
	f.IntVarP(&cfg.Flags.Limit, "limit", "L", 0, "limit the number of snapshots returned")
	f.IntVarP(&cfg.Flags.Year, "year", "Y", 0, "fetches the last N snapshots from a specific year")
	f.Bool("help", false, "help message")
	_ = f.MarkHidden("help")

	return c
}
