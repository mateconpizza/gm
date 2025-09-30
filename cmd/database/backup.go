package database

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
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
		RunE:    backupNewFunc,
	}
)

// backupLockFunc lock backups.
func backupLockFunc(_ *cobra.Command, _ []string) error {
	app := config.New()
	fs, err := handler.SelectBackupMany(app.Path.Backup, "select backup/s to lock")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightGray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
	)

	cgi := func(s string) string { return color.BrightGray(s).Italic().String() }

	c.F.Header(fmt.Sprintf("locking %d backups\n", len(fs))).Row("\n").Flush()

	for _, r := range fs {
		if err := handler.LockRepo(c, r); err != nil {
			if errors.Is(err, terminal.ErrActionAborted) || errors.Is(err, terminal.ErrIncorrectAttempts) {
				c.F.Warning(cgi("skipped: " + err.Error() + "\n")).Flush()
				continue
			}

			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// backupUnlockFunc unlock backups.
func backupUnlockFunc(_ *cobra.Command, _ []string) error {
	app := config.New()
	p := app.Path.Backup

	if !files.Exists(p) {
		return fmt.Errorf("%w", db.ErrBackupNotFound)
	}

	repos, err := handler.SelectFileLocked(p, "select backup to unlock")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightGray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
	)

	return handler.UnlockRepo(c, repos[0])
}

// backupNewFunc create a new backup.
func backupNewFunc(_ *cobra.Command, _ []string) error {
	app := config.New()

	r, err := db.New(app.DBPath)
	if err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	defer r.Close()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	srcPath := app.DBPath
	if !files.Exists(srcPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, srcPath)
	}

	if files.Empty(srcPath) {
		return fmt.Errorf("%w", db.ErrDBEmpty)
	}
	fmt.Print(summary.Info(c, r, app.Path.Backup))

	c.F.Reset().Row("\n").Flush()

	cgb := func(s string) string { return color.BrightGreen(s).Italic().String() }
	if !app.Flags.Force {
		if err := c.ConfirmErr("create "+cgb("backup"), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := files.MkdirAll(app.Path.Backup); err != nil {
		return fmt.Errorf("%w", err)
	}

	newBkPath, err := dbtask.Backup(r.Cfg.Fullpath(), app.Path.Backup)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("backup created: %q\n", filepath.Base(newBkPath))))

	if app.Flags.Force {
		slog.Debug("skipping lock", "path", newBkPath)
		return nil
	}

	return handler.LockRepo(c, newBkPath)
}

// backupPrettyPrint pretty repo info.
// func backupPrettyPrint(cmd *cobra.Command, args []string) error {
// 	r, err := db.New(config.App.DBPath)
// 	if err != nil {
// 		return fmt.Errorf("backup: %w", err)
// 	}
// 	defer r.Close()
//
// 	c := ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))))
// 	fmt.Print(summary.Info(c, r))
//
// 	return nil
// }
