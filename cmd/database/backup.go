package database

import (
	"context"
	"fmt"

	files "github.com/mateconpizza/gofiles"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/pkg/db"
)

func newBackupCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "backup",
		Aliases:     []string{"b", "bk"},
		Short:       "backup management",
		Annotations: cli.SkipGitSync,
	}

	c.AddCommand(
		newBackupAddCmd(app),
		newBackupListCmd(app),
		newBackupRemoveCmd(app),
		newBackupLockCmd(app),
		newBackupUnlockCmd(app),
	)

	return c
}

func newBackupAddCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "create",
		Short:   "create a new backup",
		Aliases: []string{"add", "new", "create"},
		Example: app.Example(`  $ {cmd} db backup create
  $ {cmd} db backup new
  $ {cmd} db backup add --db work`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, dbops.NewBackup)
		},
	}

	return c
}

func newBackupLockCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "lock",
		Short: "lock a backup",
		Example: app.Example(`  $ {cmd} db backup lock
  $ {cmd} db backup lock --db work`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, dbops.LockBackup)
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
			return cmdutil.Run(cmd, args, func(ctx context.Context, d *deps.Deps) error {
				if !files.Exists(app.Path.Backup()) {
					return db.ErrBackupNotFound
				}

				repo, err := dbops.SelectEncrypted(cmd.Context(), d, app.Path.Backup())
				if err != nil {
					return fmt.Errorf("%w", err)
				}

				return dbops.Unlock(cmd.Context(), d, repo)
			})
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
