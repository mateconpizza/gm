// Package archive provides commands for querying the Internet Archive Wayback
// Machine to retrieve historical snapshots of bookmarked URLs.
package archive

import (
	"time"

	menu "github.com/mateconpizza/go-fzf"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "archive [query]",
		Aliases: []string{"snap", "ar", "wm", "wayback"},
		Short:   "show archive URL",
		Example: app.Example(`  $ {cmd} url archive <query>
  $ {cmd} url archive --menu
  $ {cmd} url archive --tag golang`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(
				cmd,
				args,
				setupMenu(app),
				handler.WaybackList,
				handler.WithSnapshots,
			)
		},
	}
	cmdutil.FlagsFilter(c, app)
	cmdutil.FlagMenu(c, app)
	c.AddCommand(newLookupCmd(app), newSaveCmd(app))

	return c
}

func newLookupCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "fetch",
		Short:   "wayback lookup",
		Aliases: []string{"get"},
		Example: app.Example(`  $ {cmd} url archive fetch 179 --latest
  $ {cmd} url archive fetch 179 --limit 5
  $ {cmd} url archive fetch 179 --limit 5 --year 2023
  $ {cmd} url archive get --menu
  $ {cmd} url archive fetch 179 --timeout 45s`),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app,
				menu.WithKeybinds(
					menu.KeymapTogglePreview(),
					menu.KeymapToggleAll(),
				),
			)
			return cmdutil.Execute(cmd, args, m, handler.WaybackLookup)
		},
	}

	f := c.Flags()
	f.SortFlags = false
	f.BoolVarP(&app.Flags.Update, "latest", "l", false, "fetches lasts snapshot from Wayback Machine")
	f.IntVarP(&app.Flags.Limit, "limit", "L", 0, "return at most N snapshots")
	f.IntVarP(&app.Flags.Year, "year", "Y", 0, "restrict snapshots to a specific year")
	f.DurationVar(&app.Flags.Timeout, "timeout", 30*time.Second, "maximum time to wait for snapshot retrieval")

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.FlagOutput(c, app, app.Format, formatter.ValidFormats())

	return c
}

func newSaveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "add",
		Short: "wayback request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(
				cmd,
				args,
				setupMenu(app),
				handler.WaybackMakeSnapshot,
				handler.WithoutSnapshots,
			)
		},
	}
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	return c
}

func setupMenu(app *application.App, opts ...menu.Option) *menu.Menu[bookmark.Bookmark] {
	fm, _ := formatter.New(formatter.ArchiveURL)
	p := fm.Menu.Placeholder()
	return picker.NewWithFormatter(app, fm, append(opts,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(" wayback machine "),
		menu.WithHeaderKeymaps(),
		menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), app.DBBaseName(), p.Single())),
	)...)
}
