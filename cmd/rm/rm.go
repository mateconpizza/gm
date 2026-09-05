package rm

import (
	menu "github.com/mateconpizza/go-fzf"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/formatter"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm [query]",
		Aliases: []string{"remove"},
		Short:   "remove bookmark",
		Example: app.Example(`  $ {cmd} rm <id> or <query>
  $ {cmd} rm --menu --sort favorite
  $ {cmd} rm --tag golang,awesome
  $ {cmd} rm --tag golang --tag awesome`),
		RunE: func(cmd *cobra.Command, args []string) error {
			fm := app.Formatter()

			m := picker.NewWithFormatter(
				app,
				fm,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" deletion "),
				menu.WithHeaderKeymaps(),
				menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), app.DBBaseName(), fm.Menu.Placeholder().Single())),
				menu.WithKeybinds(menu.KeymapToggleAll(), menu.KeymapTogglePreview()),
			)

			return cmdutil.Execute(cmd, args, m, handler.Remove)
		},
	}
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagOutput(c, app, app.Format, formatter.ValidFormats())
	cmdutil.FlagsFilter(c, app)

	c.AddCommand(
		newDatabaseRemoveCmd(app),
		newBackupRemoveCmd(app),
	)
	return c
}

func newBackupRemoveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "backup",
		Short:   "remove one or more backups",
		Aliases: []string{"b", "bk"},
		Example: app.Example(`  $ {cmd} rm backup
  $ {cmd} rm bk --db work
  $ {cmd} rm backup --yes`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, dbops.RemoveBackups)
		},
	}

	c.Flags().BoolVar(&app.Flags.All, "show-all", false, "show all backups")

	return c
}

func newDatabaseRemoveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "database",
		Aliases: []string{"db", "d"},
		Short:   "remove a database",
		Example: app.Example(`  $ {cmd} rm db --db {db}
  $ {cmd} rm db --yes`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, handler.RemoveRepos)
		},
	}

	cmdutil.FlagMenu(c, app)

	return c
}
