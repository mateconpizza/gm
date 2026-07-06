package archive

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "archive [query]",
		Aliases: []string{"snap", "ar", "a"},
		Short:   "show archive URL",
		Example: app.Example(`  $ {cmd} url archive <query>
  $ {cmd} url archive --menu
  $ {cmd} url archive --tag golang`),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				if len(bs) == 0 {
					slog.Debug("URL archive: no items found")
					return sys.ErrExitFailure
				}

				var sb strings.Builder
				for _, u := range bs {
					sb.WriteString(u.ArchiveURL)
					sb.WriteByte('\n')
				}
				fmt.Fprint(d.Writer(), sb.String())
				return nil
			}

			return cmdutil.Execute(cmd, args, setupMenu(app), a, onlySnapshots)
		},
	}

	cmdutil.FlagsFilter(c, app)
	cmdutil.FlagMenu(c, app)
	c.AddCommand(newLookupCmd(app), newOpenCmd(app))

	return c
}

func newOpenCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"o"},
		Short:   "open archive URL in browser",
		Hidden:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(cmd, args, setupMenu(app), handler.Open, onlySnapshots)
		},
	}
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	return c
}

func onlySnapshots(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	filtered := make([]*bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		if bs[i].ArchiveURL != "" {
			filtered = append(filtered, bs[i])
		}
	}

	if len(filtered) == 0 {
		return filtered
	}

	result := make([]*bookmark.Bookmark, 0, len(filtered))
	for i := range filtered {
		f := filtered[i]
		b := bookmark.New()
		b.Title = f.Title
		b.ID = f.ID
		b.URL = f.ArchiveURL
		b.ArchiveTimestamp = f.ArchiveTimestamp
		b.ArchiveURL = b.URL
		result = append(result, b)
	}

	return result
}

func setupMenu(app *application.App) *menu.Menu[bookmark.Bookmark] {
	fm, _ := formatter.New(formatter.ArchiveURL)

	p := fm.Menu.Placeholder()
	kb := menu.NewBindBuilder(app.Command(), app.DBBaseName()).
		WithPlaceholder(p.Multi())

	return picker.NewWithFormatter(
		app,
		fm,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(" archive URL "),
		menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), p.Single())),
		menu.WithKeybinds(kb.New("enter", "open-in-browser").Execute("url archive open")),
	)
}
