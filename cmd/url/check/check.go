// Package check provides bookmark health verification and maintenance.
package check

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCheckCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "check",
		Short: "check URLs HTTP status",
		Example: app.Example(`  $ {cmd} url check
  $ {cmd} url check -c 200,400
  $ {cmd} url check -c 2,4
  $ {cmd} url check --code 404,403`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(
				cmd,
				args,
				setupMenu(app, " bookmark status "),
				handler.HTTPStatusCheck,
				handler.HTTPStatusCodeFilter(app.Flags.Field),
			)
		},
	}

	fields := []string{"200", "300", "400", "500"}
	c.Flags().StringVarP(&app.Flags.Field, "code", "c", "", "filter status code: "+strings.Join(fields, ", "))

	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	c.AddCommand(newUpdateCmd(app))

	return c
}

func newUpdateCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "update [id|query]",
		Short: "update metadata: title, desc, tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(
				cmd,
				args,
				setupMenu(app, " update metadata "),
				handler.UpdateMetadata,
			)
		},
	}

	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}

func NewStatusCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "filter bookmarks by HTTP status code",
		Example: app.Example(`  $ {cmd} url status
  $ {cmd} url status -c 200,400
  $ {cmd} url status -c 2,4
  $ {cmd} url status --code 404,403`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(
				cmd,
				args,
				nil,
				handler.HTTPStatus,
				handler.HTTPStatusCodeFilter(app.Flags.Field),
			)
		},
	}

	fields := []string{"200", "300", "400", "500"}
	c.Flags().StringVarP(&app.Flags.Field, "code", "c", "", "filter status code: "+strings.Join(fields, ", "))

	return c
}

func setupMenu(app *application.App, label string) *menu.Menu[bookmark.Bookmark] {
	fm := app.UI.MenuFmt
	p := fm.Menu.Placeholder
	return picker.NewWithFormatter(
		app,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(label),
		menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), p)),
	)
}
