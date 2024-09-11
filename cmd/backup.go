package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/terminal"
	"github.com/haaag/gm/internal/util"
	"github.com/haaag/gm/internal/util/frame"
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

		flags := map[bool]func(r *repo.SQLiteRepository) error{
			bkCreate: backupCreate,
			bkPurge:  backupPurge,
			bkList:   printsBackupInfo,
		}

		if handler, run := flags[true]; run {
			return handler(r)
		}

		return printsBackupInfo(r)
	},
}

func init() {
	backupCmd.Flags().BoolVarP(&bkCreate, "create", "c", false, "create backup")
	backupCmd.Flags().BoolVarP(&bkList, "list", "l", false, "list backups (default)")
	backupCmd.Flags().BoolVarP(&bkPurge, "purge", "P", false, "purge execedent backups")
	backupCmd.Flags().BoolVarP(&bkDetail, "detail", "d", false, "show backup details")
	rootCmd.AddCommand(backupCmd)
}

// backupCreate creates a backup of the specified repository if
// conditions are met, including confirmation and backup limits.
func backupCreate(r *repo.SQLiteRepository) error {
	if !Force && !isBackupEnabled() {
		return repo.ErrBackupDisabled
	}

	srcPath := r.Cfg.Fullpath()
	if err := checkDBState(srcPath); err != nil {
		return err
	}
	if err := printsBackupInfo(r); err != nil {
		return err
	}

	q := fmt.Sprintf("\ncreate %s?", color.BrightGreen("backup").Bold())
	if !terminal.Confirm(q, "y") {
		return ErrActionAborted
	}

	srcName := filepath.Base(srcPath)
	destName := repo.AddPrefixDate(srcName)
	if err := repo.CreateBackup(srcPath, destName, Force); err != nil {
		return fmt.Errorf("%w", err)
	}

	success := color.BrightGreen("successfully").Italic().Bold()
	fmt.Printf("backup created %s: %s\n", success.String(), destName)

	return nil
}

// backupPurge removes old backups.
func backupPurge(r *repo.SQLiteRepository) error {
	backups, err := repo.GetBackups(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	backups = backups.TrimElements(r.Cfg.MaxBackups)
	n := backups.Len()
	if n == 0 {
		return repo.ErrBackupNoPurge
	}

	status := repo.Summary(r)
	status += repo.BackupsSummary(r)
	f := frame.New(frame.WithColorBorder(color.Gray))

	if n > 0 {
		f.Header(color.BrightRed(n, "backups to delete").Bold().Italic().String())
		backups.ForEach(func(b string) {
			f.Row(repo.SummaryRecords(r, b))
		})

		status += f.String()
	}

	fmt.Println(status)
	nPurgeStr := color.BrightRed("purge").Bold().String()
	if !Force && !terminal.Confirm(fmt.Sprintf("%s %d backup/s?", nPurgeStr, n), "n") {
		return ErrActionAborted
	}

	formatter := func(s string) error {
		fmt.Println(s)

		return nil
	}

	if err := backups.ForEachErr(formatter); err != nil {
		return fmt.Errorf("removing backup: %w", err)
	}

	success := color.BrightGreen("successfully").Italic().Bold()
	fmt.Println("backups purged", success.String())

	return nil
}

// printsBackupInfo prints repository's backup info.
func printsBackupInfo(r *repo.SQLiteRepository) error {
	s := repo.Summary(r)
	s += repo.BackupsSummary(r)
	s += repo.BackupDetail(r)
	fmt.Print(s)

	return nil
}

// getMaxBackup loads the max backups allowed from a env var defaults to 3.
func getMaxBackup() int {
	n := config.DB.BackupMaxBackups
	defaultMax := strconv.Itoa(n)
	maxBackups, err := strconv.Atoi(util.GetEnv(config.App.Env.BackupMax, defaultMax))
	if err != nil {
		return n
	}

	return maxBackups
}

func isBackupEnabled() bool {
	return getMaxBackup() > 0
}
