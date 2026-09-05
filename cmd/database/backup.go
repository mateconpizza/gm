package database

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/ui"
)

func newBackupCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "backup",
		Aliases:     []string{"b", "bk"},
		Short:       "backup management",
		Annotations: cli.SkipGitSync,
	}

	c.AddCommand(
		newBackupListCmd(app),
		newBackupRemoveCmd(app),
		newBackupLockCmd(app),
		newBackupUnlockCmd(app),
	)

	return c
}

func newBackupLockCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "lock",
		Short: "lock a backup",
		Example: app.Example(`  $ {cmd} db backup lock
  $ {cmd} db backup lock --db work`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbops.LockBackup(cmd.Context(), app, ui.DefaultConsole)
		},
	}

	return c
}

func newBackupUnlockCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "unlock",
		Short: "unlock a database backup",
		Example: app.Example(`  $ {cmd} db backup unlock
  $ {cmd} db backup unlock --db work`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbops.UnlockBackup(cmd.Context(), app, ui.DefaultConsole)
		},
	}

	return c
}

func newBackupListCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "list",
		Short:   "list backups",
		Aliases: []string{"l", "ls", "info", "i"},
		Example: app.Example(`  $ {cmd} db backup list
  $ {cmd} db backup list --db work
  $ {cmd} db backup ls`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, dbops.BackupList)
		},
	}

	cmdutil.HideFlag(c, "yes", "force")

	return c
}
