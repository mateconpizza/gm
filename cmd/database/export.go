package database

import (
	"context"
	"os"
	"strings"

	menu "github.com/mateconpizza/go-fzf"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

func newExportCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "export [id|query]",
		Short:   "export bookmarks",
		Aliases: []string{"ex", "x"},
	}

	cmds := []func(*application.App) *cobra.Command{
		newExportHTMLCmd,
		newExportJSONCmd,
		newExportCSVCmd,
		newExportMarkdownCmd,
	}

	for i := range cmds {
		cmd := cmds[i](app)
		cmdutil.FlagSort(cmd, app, bookmark.SortSupported)
		cmdutil.FlagMenu(cmd, app)
		cmdutil.FlagsFilter(cmd, app)
		cmdutil.FlagOutput(cmd, app, app.Format(), formatter.ValidFormats())
		c.AddCommand(cmd)
	}
	return c
}

func newExportHTMLCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "html [id|query]",
		Short: "export to HTML Netscape",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " export to HTML ")
			return cmdutil.Execute(cmd, args, m, func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				return bookio.ExportToNetscapeHTML(bs, os.Stdout)
			})
		},
	}
	return c
}

func newExportJSONCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "json [id|query]",
		Short: "export to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " export to JSON ")
			return cmdutil.Execute(cmd, args, m, func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				return printer.RecordsJSON(os.Stdout, bs)
			})
		},
	}
	return c
}

func newExportCSVCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "csv [id|query]",
		Short: "export to CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " export to CSV ")
			return cmdutil.Execute(cmd, args, m, func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				return bookio.ExportToCSV(bs, os.Stdout, bookio.ParseCSVFields(app.Flags.Field))
			})
		},
	}
	cmdutil.FlagFields(c, app, "all,"+cmdutil.WrapFields(bookmark.Fields(), ",", 50))
	return c
}

func newExportMarkdownCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "markdown [id|query]",
		Short:   "export to Markdown",
		Aliases: []string{"readme", "md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " export to Markdown ")
			return cmdutil.Execute(cmd, args, m, func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				app.Git.Readme = &git.Readme{
					Enabled: true,
					Glyphs:  app.Flags.Glyphs,
					Summary: app.Flags.Summary,
				}

				return gitops.NewReadmeFile(ctx, app, os.Stdout, bs)
			})
		},
	}

	flagsMarkdown(c, app)

	return c
}

func setupMenu(app *application.App, label string) *menu.Menu[*bookmark.Bookmark] {
	fm := app.Formatter()
	p := fm.Menu.Placeholder()

	kb := menucfg.NewBindBuilder().
		WithCommand(app.Command()).
		WithDBName(app.DBBaseName()).
		WithPlaceholder(p.Multi())

	k := app.Menu.Keymaps()
	keys := []*menu.Keymap{
		kb.Builtin(k.ToggleAll, menu.KeybindActionToggleAll),
		kb.Builtin(k.Preview, menu.KeybindActionTogglePreview),
		kb.NewKeymap().WithBind(menu.KeyTab).WithDesc("toggle-select"),
	}

	return picker.NewWithFormatter(
		app,
		fm,
		menu.WithMultiSelection(),
		menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), app.DBBaseName(), p.Single())),
		menu.WithPreviewWindow(picker.PreviewWindowArg(app.Menu.Preview)),
		menu.WithKeybinds(keys...),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(label),
		menu.WithHeaderKeymaps(),
	)
}

func flagCompletion(c *cobra.Command, flag string, items []string) {
	cmdutil.FlagCompletion(c, flag, func(cmd *cobra.Command) ([]string, error) {
		return items, nil
	})
}

func flagsMarkdown(c *cobra.Command, app *application.App) {
	f := c.Flags()

	f.StringVar(&app.Flags.Summary, "summary", bookio.DefaultSummary, strings.Join(bookio.SupportedSummary, ", "))
	flagCompletion(c, "summary", bookio.SupportedSummary)

	f.StringVar(&app.Flags.Glyphs, "glyphs", bookio.DefaultGlyphs, strings.Join(bookio.SupportedGlyphs, ", "))
	flagCompletion(c, "glyphs", bookio.SupportedGlyphs)

	f.StringVar(&app.Flags.Type, "type", bookio.DefaultType, "")
	flagCompletion(c, "type", bookio.SupportedTypes)
	_ = f.MarkHidden("type")

	c.PreRunE = cli.ChainHooks(
		cli.HookValidateFlags(&app.Flags.Summary, bookio.SupportedSummary),
		cli.HookValidateFlags(&app.Flags.Glyphs, bookio.SupportedGlyphs),
		cli.HookValidateFlags(&app.Flags.Type, bookio.SupportedTypes),
	)
}
