package database

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/repository"
)

func init() {
	cfg := config.App
	backupUnlockCmd.Flags().
		BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "select a backup to lock|unlock (fzf)")
	backupCmd.AddCommand(BackupNewCmd, backupListCmd, backupRmCmd, backupLockCmd, backupUnlockCmd)
	dbRootCmd.AddCommand(backupCmd)
}

// backupCmd backup management.
var (
	backupCmd = &cobra.Command{
		Use:     "backup",
		Aliases: []string{"b", "bk", "backups"},
		Short:   "Backup management",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
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

	// backupListCmd list backups.
	backupListCmd = &cobra.Command{
		Use:     "list",
		Short:   "List backups from a database",
		Aliases: []string{"ls", "l", "i", "info"},
		RunE:    backupPrettyPrint,
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
func backupLockFunc(cmd *cobra.Command, args []string) error {
	fs, err := handler.SelectBackupMany(config.App.Path.Backup, "select backup/s to lock")
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
func backupUnlockFunc(cmd *cobra.Command, args []string) error {
	if !files.Exists(config.App.Path.Backup) {
		return fmt.Errorf("%w", db.ErrBackupNotFound)
	}

	repos, err := handler.SelectFileLocked(config.App.Path.Backup, "select backup to unlock")
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
func backupNewFunc(cmd *cobra.Command, args []string) error {
	r, err := repository.New(config.App.DBPath)
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

	srcPath := config.App.DBPath
	if !files.Exists(srcPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, srcPath)
	}

	if files.Empty(srcPath) {
		return fmt.Errorf("%w", db.ErrDBEmpty)
	}
	fmt.Print(summary.Info(c, r))

	c.F.Reset().Row("\n").Flush()

	cgb := func(s string) string { return color.BrightGreen(s).Italic().String() }
	if !config.App.Flags.Force {
		if err := c.ConfirmErr("create "+cgb("backup"), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := files.MkdirAll(config.App.Path.Backup); err != nil {
		return fmt.Errorf("%w", err)
	}

	newBkPath, err := dbtask.Backup(r.Fullpath())
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("backup created: %q\n", filepath.Base(newBkPath))))

	if config.App.Flags.Force {
		slog.Debug("skipping lock", "path", newBkPath)
		return nil
	}

	return handler.LockRepo(c, newBkPath)
}

// backupPrettyPrint pretty repo info.
func backupPrettyPrint(cmd *cobra.Command, args []string) error {
	r, err := repository.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	defer r.Close()

	c := ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))))
	fmt.Print(summary.Info(c, r))

	return nil
}
