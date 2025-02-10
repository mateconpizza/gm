package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
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

// backupListCmd list backups.
var backupListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List backups from a database",
	Aliases: []string{"ls", "l"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		defer r.Close()
		backupInfoPrint(r)

		return nil
	},
}

// backupCmd backup management.
var backupNewCmd = &cobra.Command{
	Use:     "new",
	Short:   "Create a new backup",
	Aliases: []string{"create", "new", "add"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		defer r.Close()
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		srcPath := r.Cfg.Fullpath()
		if !files.Exists(srcPath) {
			return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, srcPath)
		}
		if files.Empty(srcPath) {
			return fmt.Errorf("%w", repo.ErrDBEmpty)
		}
		backupInfoPrint(r)
		f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
		f.Row("\n").Render().Clean()
		c := color.BrightGreen("backup").Bold().String()
		if !t.Confirm(f.Success("create "+c).String(), "n") {
			return handler.ErrActionAborted
		}
		newBkPath, err := repo.NewBackup(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		success := color.BrightGreen("Successfully").Italic().String()
		s := color.Text(newBkPath).Italic().String()
		t.ReplaceLine(1, f.Clean().Success(success+" backup created: "+s).String())

		return nil
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

// backupInfoPrint prints repository's backup info.
func backupInfoPrint(r *repo.SQLiteRepository) {
	s := repo.RepoSummary(r)
	s += repo.BackupsSummary(r)
	s += repo.BackupSummaryDetail(r)
	fmt.Print(s)
}

func init() {
	f := backupCmd.Flags()
	f.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	f.BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	f.StringVarP(&DBName, "name", "n", config.DefaultDBName, "database name")
	f.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	_ = backupCmd.Flags().MarkHidden("color")
	f.BoolP("help", "h", false, "Hidden help")
	_ = f.MarkHidden("help")
	backupCmd.AddCommand(backupNewCmd, backupListCmd, backupRmCmd)
	rootCmd.AddCommand(backupCmd)
}
