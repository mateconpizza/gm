package database

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

func newBackupCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "backup",
		Aliases:     []string{"b", "bk"},
		Short:       "backup management",
		RunE:        cli.HookHelp,
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
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return dbops.NewBackup(cmd.Context(), d)
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
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return dbops.LockBackup(cmd.Context(), d)
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
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			if !files.Exists(app.Path.Backup()) {
				return fmt.Errorf("%w", db.ErrBackupNotFound)
			}

			repos, err := handler.SelectFileLocked(cmd.Context(), d, app.Path.Backup(), "select backup to unlock")
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return dbops.Unlock(cmd.Context(), d, repos[0])
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
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			p, f := d.Console().Palette(), d.Console().Frame()
			r, err := d.Repository()
			if err != nil {
				return err
			}

			title := p.BrightMagenta.With(p.Bold).
				Sprint("Repository Backups")

			subtitle := p.Dim.With(p.Italic).
				Sprint("latest backup snapshots")

			name := p.BrightYellow.With(p.Bold).
				Sprint(files.StripSuffixes(r.Name()))

			repo := p.Dim.With(p.Italic).
				Sprint("repo: " + name)

			info := p.Dim.With(p.Italic).
				Sprintf(" (%d bookmarks)", r.Count(cmd.Context(), "bookmarks"))

			f.Headerln(title).
				Headerln(subtitle).
				Rowln().
				Midln(repo + info).
				Rowln().Flush()

			bkDetail, err := summary.BackupListDetail(cmd.Context(), d, true)
			if err != nil {
				return err
			}

			fmt.Fprint(d.Writer(), bkDetail)

			return nil
		},
	}

	cmdutil.HideFlag(c, "yes", "force")

	return c
}
