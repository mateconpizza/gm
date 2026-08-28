package database

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/handler"
)

func newBackupRemoveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm",
		Short:   "remove one or more backups",
		Aliases: []string{"backup", "b", "backups"},
		Example: app.Example(`  $ {cmd} db backup rm
  $ {cmd} db backup rm --db work
  $ {cmd} db backup rm --yes`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, dbops.RemoveBackups)
		},
	}

	return c
}

func newDatabaseRemoveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm",
		Aliases: []string{"remove"},
		Short:   "remove a database",
		Example: app.Example(`  $ {cmd} db rm --db {db}`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, handler.RemoveAndUntrack)
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}
