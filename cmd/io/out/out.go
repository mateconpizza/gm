package out

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "export [id|query]",
		Short:   "export bookmarks",
		Aliases: []string{"e", "ext"},
		RunE:    cli.HookHelp,
	}

	cmds := []*cobra.Command{
		newHTMLCmd(app),
		newJSONCmd(app),
		newCSVCmd(app),
	}

	for i := range cmds {
		cmdutil.HideFlag(cmds[i], "help")
	}

	c.AddCommand(cmds...)
	cmdutil.HideFlag(c, "help")

	return c
}

func newHTMLCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "html [id|query]",
		Short: "export bookmarks",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" export to HTML "),
				menu.WithPreview(app.PreviewCmd(app.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(_ *deps.Deps, bs []*bookmark.Bookmark) error {
				return bookio.ExportToNetscapeHTML(bs, os.Stdout)
			})
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}

func newJSONCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "json [id|query]",
		Short: "export to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" export to JSON "),
				menu.WithPreview(app.PreviewCmd(app.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(_ *deps.Deps, bs []*bookmark.Bookmark) error {
				return printer.RecordsJSON(bs)
			})
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}

func newCSVCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "csv [id|query]",
		Short: "export to CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented yet...")
			return cmd.Help()
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}
