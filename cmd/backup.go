package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"

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

// Subcommand Flags.
var bkCreate, bkList, bkPurge, bkDetail bool

// backupCmd backup management.
var backupCmd = &cobra.Command{
	Use:     "bk",
	Aliases: []string{"backup"},
	Short:   "backup management",
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

		flags := map[bool]func(t *terminal.Term, r *repo.SQLiteRepository) error{
			bkCreate: backupCreate,
			bkPurge:  backupPurge,
			bkList:   backupInfoPrint,
		}

		if handler, run := flags[true]; run {
			return handler(t, r)
		}

		return backupInfoPrint(t, r)
	},
}

func init() {
	f := backupCmd.Flags()
	f.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	f.BoolVarP(&Remove, "remove", "r", false, "remove a backup")
	f.BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	f.StringVarP(&DBName, "name", "n", config.DB.Name, "database name")
	f.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	// actions
	f.BoolVarP(&bkCreate, "create", "c", false, "create backup")
	f.BoolVarP(&bkList, "list", "l", false, "list backups")
	f.BoolVarP(&bkPurge, "purge", "P", false, "purge execedent backups")
	f.BoolVarP(&bkDetail, "detail", "d", false, "show backup details")

	_ = backupCmd.Flags().MarkHidden("color")
	rootCmd.AddCommand(backupCmd)
}

// backupCreate creates a backup of the specified repository if
// conditions are met, including confirmation and backup limits.
func backupCreate(t *terminal.Term, r *repo.SQLiteRepository) error {
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
	if err := backupInfoPrint(t, r); err != nil {
		return err
	}

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
}

// backupPurge removes old backups.
func backupPurge(t *terminal.Term, r *repo.SQLiteRepository) error {
	backups, err := repo.Backups(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	backups = backups.TrimElements(r.Cfg.Backup.Limit)
	n := backups.Len()
	if n == 0 {
		return repo.ErrBackupNoPurge
	}

	status := repo.Summary(r)
	status += repo.BackupsSummary(r)
	f := frame.New(frame.WithColorBorder(color.BrightGray))

	if n > 0 {
		f.Header(color.BrightRed("files to delete").Italic().String())
		backups.ForEach(func(b string) {
			f.Row(repo.SummaryRecords(b))
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

	if err := backups.ForEachErr(files.Remove); err != nil {
		return fmt.Errorf("removing backup: %w", err)
	}

	t.ClearLine(1)
	success := color.BrightGreen("Successfully").Italic().String()
	f.Clean().Success(success + " backups purged").Ln().Render()

	return nil
}

// backupInfoPrint prints repository's backup info.
func backupInfoPrint(_ *terminal.Term, r *repo.SQLiteRepository) error {
	s := repo.Summary(r)
	s += repo.BackupsSummary(r)
	s += repo.BackupDetail(r)
	fmt.Print(s)

	return nil
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

// importSelectBackup prompts the user to select a backup.
