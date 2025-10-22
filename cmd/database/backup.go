package database

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// backupCmd backup management.
var (
	backupCmd = &cobra.Command{
		Use:     "backup",
		Aliases: []string{"b", "bk", "backups"},
		Short:   "Backup management",
		RunE:    cli.HookHelp,
	}

	// backupRmCmd remove backups.
	backupRmCmd = &cobra.Command{
		Use:   "rm",
		Short: "Remove a backup/s",
		RunE:  bkRemoveCmd.RunE,
	}

	// backupRmCmd remove backups.
	backupLockCmd = &cobra.Command{
		Use:   "lock",
		Short: "Lock a database backup",
		RunE:  backupLockFunc,
	}

	// backupRmCmd remove backups.
	backupUnlockCmd = &cobra.Command{
		Use:   "unlock",
		Short: "Unlock a database backup",
		RunE:  backupUnlockFunc,
	}

	// backupCmd backup management.
	BackupNewCmd = &cobra.Command{
		Use:     "new",
		Short:   "Create a new backup",
		Aliases: []string{"create", "add"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.New()
			r, err := db.New(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("backup: %w", err)
			}
			defer r.Close()

			return backupNewFunc(app.New(cmd.Context(),
				app.WithConfig(cfg),
				app.WithDB(r),
				app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
					r.Close()
					sys.ErrAndExit(err)
				})),
			))
		},
	}
)

// backupLockFunc lock backups.
func backupLockFunc(cmd *cobra.Command, _ []string) error {
	a := app.New(cmd.Context(),
		app.WithConfig(config.New()),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
	)
	fs, err := handler.SelectBackupMany(a, a.Cfg.Path.Backup, "select backup/s to lock")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := a.Console()
	f := c.Frame()
	f.Header(fmt.Sprintf("locking %d backups\n", len(fs))).Row("\n").Flush()

	for _, r := range fs {
		if err := handler.LockRepo(a, r); err != nil {
			if errors.Is(err, sys.ErrActionAborted) || errors.Is(err, terminal.ErrIncorrectAttempts) {
				f.Warning(c.Palette().BrightGrayItalic("skipped: " + err.Error() + "\n")).Flush()
				continue
			}

			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// backupUnlockFunc unlock backups.
func backupUnlockFunc(cmd *cobra.Command, _ []string) error {
	a := app.New(cmd.Context(),
		app.WithConfig(config.New()),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
	)

	p := a.Cfg.Path.Backup
	if !files.Exists(p) {
		return fmt.Errorf("%w", db.ErrBackupNotFound)
	}

	repos, err := handler.SelectFileLocked(a, p, "select backup to unlock")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return handler.UnlockRepo(a, repos[0])
}

// backupNewFunc create a new backup.
func backupNewFunc(a *app.Context) error {
	r, cfg := a.DB, a.Cfg
	srcPath := cfg.DBPath
	if !files.Exists(srcPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, srcPath)
	}

	if files.Empty(srcPath) {
		return fmt.Errorf("%w", db.ErrDBEmpty)
	}
	fmt.Fprint(a.Writer(), summary.Info(a))

	c := a.Console()
	f := c.Frame()
	f.Reset().Row("\n").Flush()

	if !cfg.Flags.Yes {
		if err := c.ConfirmErr("create "+c.Palette().BrightGreenItalic("backup"), "y"); err != nil {
			return err
		}
	}

	if err := files.MkdirAll(cfg.Path.Backup); err != nil {
		return err
	}

	newBkPath, err := r.Backup(a.Ctx, cfg.Path.Backup)
	if err != nil {
		return err
	}

	fmt.Fprintln(a.Writer(), c.SuccessMesg(fmt.Sprintf("backup created: %q", filepath.Base(newBkPath))))

	if cfg.Flags.Force {
		slog.Debug("skipping lock", "path", newBkPath)
		return nil
	}

	return handler.LockRepo(a, newBkPath)
}
