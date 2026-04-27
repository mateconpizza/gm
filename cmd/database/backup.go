package database

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

func newBackupCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "backup",
		Aliases: []string{"b", "bk"},
		Short:   "backup management",
		RunE:    cli.HookHelp,
	}

	c.AddCommand(newBackupAddCmd(app), newBackupRemoveCmd(app),
		newBackupLockCmd(app), newBackupUnlockCmd(app))

	return c
}

func newBackupAddCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "create",
		Short:   "create a new backup",
		Aliases: []string{"add", "new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return backupNewFunc(d)
		},
	}

	return c
}

func newBackupLockCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "lock",
		Short: "lock a database backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			fs, err := handler.SelectBackupMany(d, d.App.Path.Backup, "select backup/s to lock")
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			c := d.Console()
			f, p := c.Frame(), c.Palette()
			f.Header(fmt.Sprintf("locking %d backups\n", len(fs))).Row("\n").Flush()

			for _, r := range fs {
				if err := handler.LockRepo(d, r); err != nil {
					if errors.Is(err, sys.ErrActionAborted) || errors.Is(err, terminal.ErrIncorrectAttempts) {
						f.Warning(p.BrightBlack.With(p.Italic).Sprintf("skipped: %s\n", err.Error())).Flush()
						continue
					}

					return fmt.Errorf("%w", err)
				}
			}

			return nil
		},
	}

	return c
}

func newBackupUnlockCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "unlock",
		Short: "unlock a database backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			p := d.App.Path.Backup
			if !files.Exists(p) {
				return fmt.Errorf("%w", db.ErrBackupNotFound)
			}

			repos, err := handler.SelectFileLocked(d, p, "select backup to unlock")
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return handler.UnlockRepo(d, repos[0])
		},
	}

	return c
}

func backupNewFunc(d *deps.Deps) error {
	app := d.App
	srcPath := app.Path.Database
	if !files.Exists(srcPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, srcPath)
	}

	if files.Empty(srcPath) {
		return fmt.Errorf("%w", db.ErrDBEmpty)
	}
	fmt.Fprint(d.Writer(), summary.Info(d))

	c := d.Console()
	f, p := c.Frame(), c.Palette()
	f.Reset().Row("\n").Flush()

	if !app.Flags.Yes {
		if err := c.ConfirmErr("create "+p.BrightGreen.Wrap("backup", p.Italic), "y"); err != nil {
			return err
		}
	}

	if err := files.MkdirAll(app.Path.Backup); err != nil {
		return err
	}

	newBkPath, err := d.Repo.Backup(d.Context(), app.Path.Backup)
	if err != nil {
		return err
	}

	fmt.Fprintln(d.Writer(), c.SuccessMesg(fmt.Sprintf("backup created: %q", filepath.Base(newBkPath))))

	if app.Flags.Force {
		slog.Debug("skipping lock", "path", newBkPath)
		return nil
	}

	return nil
}
