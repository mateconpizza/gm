package out

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "export [id|query]",
		Short:   "export bookmarks",
		Aliases: []string{"e", "ext"},
		RunE:    cli.HookHelp,
	}

	cmds := []*cobra.Command{
		newHTMLCmd(cfg),
		newJSONCmd(cfg),
		newCSVCmd(cfg),
	}

	for i := range cmds {
		cmdutil.HideFlag(cmds[i], "help")
	}

	c.AddCommand(cmds...)
	cmdutil.HideFlag(c, "help")

	return c
}

func newHTMLCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "html [id|query]",
		Short: "export bookmarks",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" export to HTML "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(_ *app.Context, bs []*bookmark.Bookmark) error {
				return bookio.ExportToNetscapeHTML(bs, os.Stdout)
			})
		},
	}

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)

	return c
}

func newJSONCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "json [id|query]",
		Short: "export to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" export to JSON "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(_ *app.Context, bs []*bookmark.Bookmark) error {
				return printer.RecordsJSON(bs)
			})
		},
	}

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)

	return c
}

func newCSVCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "csv [id|query]",
		Short: "export to CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented yet...")
			return cmd.Help()
		},
	}

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)

	return c
}
