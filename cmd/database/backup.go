package database

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

func newBackupCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "backup",
		Aliases: []string{"b", "bk"},
		Short:   "backup management",
		RunE:    cli.HookHelp,
	}

	c.AddCommand(newBackupAddCmd(cfg), newBackupRemoveCmd(cfg),
		newBackupLockCmd(cfg), newBackupUnlockCmd(cfg))

	return c
}

func newBackupAddCmd(_ *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "create",
		Short:   "create a new backup",
		Aliases: []string{"add", "new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return backupNewFunc(a)
		},
	}

	return c
}

func newBackupLockCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "lock",
		Short: "lock a database backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			fs, err := handler.SelectBackupMany(a, a.Cfg.Path.Backup, "select backup/s to lock")
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			c := a.Console()
			f, p := c.Frame(), c.Palette()
			f.Header(fmt.Sprintf("locking %d backups\n", len(fs))).Row("\n").Flush()

			for _, r := range fs {
				if err := handler.LockRepo(a, r); err != nil {
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

	cmdutil.FlagMenu(c, cfg)

	return c
}

func newBackupUnlockCmd(_ *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "unlock",
		Short: "unlock a database backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			p := a.Cfg.Path.Backup
			if !files.Exists(p) {
				return fmt.Errorf("%w", db.ErrBackupNotFound)
			}

			repos, err := handler.SelectFileLocked(a, p, "select backup to unlock")
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return handler.UnlockRepo(a, repos[0])
		},
	}

	return c
}

func backupNewFunc(a *app.Context) error {
	cfg := a.Cfg
	srcPath := cfg.DBPath
	if !files.Exists(srcPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, srcPath)
	}

	if files.Empty(srcPath) {
		return fmt.Errorf("%w", db.ErrDBEmpty)
	}
	fmt.Fprint(a.Writer(), summary.Info(a))

	c := a.Console()
	f, p := c.Frame(), c.Palette()
	f.Reset().Row("\n").Flush()

	if !cfg.Flags.Yes {
		if err := c.ConfirmErr("create "+p.BrightGreen.Wrap("backup", p.Italic), "y"); err != nil {
			return err
		}
	}

	if err := files.MkdirAll(cfg.Path.Backup); err != nil {
		return err
	}

	newBkPath, err := a.DB.Backup(a.Context(), cfg.Path.Backup)
	if err != nil {
		return err
	}

	fmt.Fprintln(a.Writer(), c.SuccessMesg(fmt.Sprintf("backup created: %q", filepath.Base(newBkPath))))

	if cfg.Flags.Force {
		slog.Debug("skipping lock", "path", newBkPath)
		return nil
	}

	return nil
}
