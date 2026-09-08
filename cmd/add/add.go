// Package add provides Cobra subcommands for creating new entities,
// including bookmarks, databases, and backups.
package add

import (
	"context"
	"fmt"

	files "github.com/mateconpizza/gofiles"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/pkg/db"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "new",
		Short:   "new bookmark or database",
		Aliases: []string{"add"},
		Example: app.Example(`  $ {cmd} new
  $ {cmd} add <URL>
  $ {cmd} new <URL> --title <title>
  $ {cmd} new <URL> --title <title> --tags <golang,awesome>`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, func(ctx context.Context, d *deps.Deps) error {
				return handler.AddBookmark(cmd.Context(), d, args)
			})
		},
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&app.Flags.Title, "title", "", "bookmark title")
	c.Flags().StringVar(&app.Flags.TagsStr, "tags", "", "bookmark tags")
	c.AddCommand(
		newAddDatabase(app),
		newBackupAddCmd(app),
	)
	return c
}

func newAddDatabase(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "db",
		Short:       "add a database",
		Aliases:     []string{"database"},
		Example:     app.Example(`  $ {cmd} new db --db <name>`),
		Annotations: cli.SkipDBCheck,
		RunE: func(cmd *cobra.Command, args []string) error {
			if files.Exists(app.Path.DB()) {
				return fmt.Errorf("%w: %q", db.ErrDBExists, app.DBName)
			}
			return setup.InitCmd.RunE(cmd, args)
		},
		PostRunE: setup.InitCmd.PostRunE,
	}
	cmdutil.FlagDBRequired(c, app)
	return c
}

func newBackupAddCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "bk",
		Short:   "add a backup",
		Aliases: []string{"backup"},
		Example: app.Example(`  $ {cmd} new backup
  $ {cmd} new bk
  $ {cmd} new bk --db work`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, dbops.NewBackup)
		},
	}
	return c
}
