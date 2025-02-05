package cmd

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

// backupCmd backup management.
var backupCmd = &cobra.Command{
	Use:     "backup",
	Aliases: []string{"b", "bk", "backups"},
	Short:   "backup management",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

// backupListCmd list backups.
var backupListCmd = &cobra.Command{
	Use:     "list",
	Short:   "list backups",
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

// backupRemoveCmd removes backups.
var backupRemoveCmd = &cobra.Command{
	Use:     "remove",
	Short:   "remove a backup",
	Aliases: []string{"rm", "del", "delete"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		defer r.Close()
		m := menu.New[Repo](
			menu.WithDefaultSettings(),
			menu.WithMultiSelection(),
			menu.WithHeader("choose backup/s to remove", false),
			menu.WithPreviewCustomCmd(config.App.Cmd+" db -n ./backup/{1} info"),
		)
		backups, err := repo.Backups(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
		f.Header("choose backup/s to remove").Ln().Render()
		items, err := handler.SelectRepo(m, backups.Items(), nil)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		backups.Set(&items)

		msg := fmt.Sprintf("remove %d backup/s?", backups.Len())
		f.Header(msg).Ln().Render()
		// f.Header(color.BrightRed("files to delete").Italic().String()).Ln().Render()

		return nil
	},
}

// backupPurgeCmd purges execedent backups.
var backupPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "purge execedent backups",
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
		backups, err := repo.Backups(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		backupsTrimmed := backupTrimExcess(backups, r.Cfg.Backup.Limit)
		n := backupsTrimmed.Len()
		if n == 0 {
			return repo.ErrBackupNoPurge
		}
		backups.Clean()

		status := repo.Summary(r)
		status += repo.BackupsSummary(r)
		f := frame.New(frame.WithColorBorder(color.BrightGray))

		if n > 0 {
			f.Header(color.BrightRed("files to delete").Italic().String())
			backupsTrimmed.ForEach(func(r Repo) {
				f.Row(repo.SummaryRecordsLine(&r))
			})

			status += f.String()
		}

		fmt.Print(status)

		f = frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
		nPurgeStr := color.BrightRed("purge").String()
		f.Row().Ln().Render().Clean()
		q := f.Header(fmt.Sprintf("%s %d backup/s?", nPurgeStr, n)).String()

		if !Force && !t.Confirm(q, "n") {
			return handler.ErrActionAborted
		}

		if err := backupsTrimmed.ForEachErr(removeSecure); err != nil {
			return fmt.Errorf("removing backup: %w", err)
		}

		t.ClearLine(1)
		success := color.BrightGreen("Successfully").Italic().String()
		f.Clean().Success(success + " backups purged").Ln().Render()

		return nil
	},
}

// backupCmd backup management.
var backupCreateCmd = &cobra.Command{
	Use:     "new",
	Short:   "create a new backup",
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

		if !Force && !r.Cfg.Backup.Enabled {
			return repo.ErrBackupDisabled
		}

		srcPath := r.Cfg.Fullpath()
		if !files.Exists(srcPath) {
			return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, srcPath)
		}
		if files.Empty(srcPath) {
			return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, srcPath)
		}
		backupInfoPrint(r)

		f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
		c := color.BrightGreen("backup").Bold().String()
		f.Row().Ln().Render()
		if !t.Confirm(f.Clean().Header("create "+c).String(), "y") {
			return handler.ErrActionAborted
		}

		srcName := filepath.Base(srcPath)
		destName := repo.AddPrefixDate(srcName, r.Cfg.Backup.DateFormat)
		if err := repo.CreateBackup(srcPath, destName, Force); err != nil {
			return fmt.Errorf("%w", err)
		}

		t.ClearLine(1)
		success := color.BrightGreen("Successfully").Italic().String()
		f.Clean().Success(success + " backup created: " + destName).Ln().Render()

		return nil
	},
}

// backupTrimExcess removes the excess backups if the limit is
// exceeded.
func backupTrimExcess(bks *slice.Slice[Repo], n int) slice.Slice[Repo] {
	filtered := slice.New[Repo]()
	items := bks.Items()
	if len(*items) > n {
		i := *items
		i = i[:n]
		filtered.Set(&i)
	}

	return *filtered
}

func removeSecure(r Repo) error {
	time.Sleep(50 * time.Millisecond)
	log.Println("removing secure file: '" + r.Cfg.Name + "'")
	return nil
}

// backupInfoPrint prints repository's backup info.
func backupInfoPrint(r *Repo) {
	s := repo.Summary(r)
	s += repo.BackupsSummary(r)
	s += repo.SummaryBackupDetail(r)
	fmt.Print(s)
}

// backupGetLimit loads the max backups allowed from a env var defaults to 3.
func backupGetLimit() int {
	n := config.DB.BackupMaxBackups
	defaultMax := strconv.Itoa(n)
	maxBackups, err := strconv.Atoi(sys.Env(config.App.Env.BackupMax, defaultMax))
	if err != nil {
		return n
	}

	return maxBackups
}

func init() {
	f := backupCmd.Flags()
	f.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	f.BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	f.StringVarP(&DBName, "name", "n", config.DB.Name, "database name")
	f.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	_ = backupCmd.Flags().MarkHidden("color")
	f.BoolP("help", "h", false, "Hidden help")
	_ = f.MarkHidden("help")
	backupCmd.AddCommand(backupCreateCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupPurgeCmd)
	backupCmd.AddCommand(backupRemoveCmd)
	rootCmd.AddCommand(backupCmd)
}
