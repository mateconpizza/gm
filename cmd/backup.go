package cmd

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

// backupCmd backup management.
var backupCmd = &cobra.Command{
	Use:     "backup",
	Aliases: []string{"b", "bk", "backups"},
	Short:   "Backup management",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

// backupRmCmd remove backups.
var backupRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove a backup/s",
	RunE: func(cmd *cobra.Command, args []string) error {
		return bkRemoveCmd.RunE(cmd, args)
	},
}

// backupRmCmd remove backups.
var backupLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock a database backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))
		fs, err := handler.SelectBackup(config.App.Path.Backup, "select backup/s to lock")
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		f.Header(fmt.Sprintf("locking %d backups\n", len(fs))).Row("\n").Flush()
		for _, r := range fs {
			if err := handler.LockRepo(t, r); err != nil {
				if errors.Is(err, terminal.ErrActionAborted) ||
					errors.Is(err, terminal.ErrIncorrectAttempts) {
					f.Warning(color.BrightGray("skipped: " + err.Error() + "\n").Italic().String()).Flush()
					continue
				}

				return fmt.Errorf("%w", err)
			}
		}

		return nil
	},
}

// backupRmCmd remove backups.
var backupUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock a database backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))
		if !files.Exists(config.App.Path.Backup) {
			return fmt.Errorf("%w", db.ErrBackupNotFound)
		}
		r, err := handler.SelectFileLocked(config.App.Path.Backup, "select backup to unlock")
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		return handler.UnlockRepo(t, r[0])
	},
}

// backupListCmd list backups.
var backupListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List backups from a database",
	Aliases: []string{"ls", "l"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := db.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		defer r.Close()
		fmt.Print(db.Info(r))

		return nil
	},
}

// backupCmd backup management.
var backupNewCmd = &cobra.Command{
	Use:     "new",
	Short:   "Create a new backup",
	Aliases: []string{"create", "add"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := db.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		defer r.Close()
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		srcPath := config.App.DBPath
		if !files.Exists(srcPath) {
			return fmt.Errorf("%w: %q", db.ErrDBNotFound, srcPath)
		}
		if files.Empty(srcPath) {
			return fmt.Errorf("%w", db.ErrDBEmpty)
		}
		fmt.Print(db.Info(r))
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		f.Row("\n").Flush().Clear()
		c := color.BrightGreen("backup").String()
		if !config.App.Force {
			if err := t.ConfirmErr(f.Question("create "+c).String(), "y"); err != nil {
				return fmt.Errorf("%w", err)
			}
		}
		newBkPath, err := r.Backup()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		success := color.BrightGreen("Successfully").Italic().String()
		s := color.Text(newBkPath).Italic().String()
		f.Clear().Success(success + " backup created: " + s + "\n").Flush()
		// FIX: don't return -> gomarks: action aborted
		if config.App.Force {
			return nil
		}

		return handler.LockRepo(t, filepath.Join(r.Cfg.BackupDir, newBkPath))
	},
}

func init() {
	f := backupCmd.Flags()
	f.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	f.StringVarP(&DBName, "name", "n", config.DefaultDBName, "database name")
	f.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	_ = backupCmd.Flags().MarkHidden("color")
	f.BoolP("help", "h", false, "Hidden help")
	_ = f.MarkHidden("help")
	backupUnlockCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "select a backup to lock|unlock (fzf)")
	backupCmd.AddCommand(backupNewCmd, backupListCmd, backupRmCmd, backupLockCmd, backupUnlockCmd)
	rootCmd.AddCommand(backupCmd)
}
