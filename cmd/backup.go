package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/presenter"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/slice"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util/files"
	"github.com/haaag/gm/pkg/util/frame"
)

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

		flags := map[bool]func(r *Repo) error{
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
func backupCreate(r *Repo) error {
	if !Force {
		if err := checkBackupEnabled(r.Cfg.MaxBackups); err != nil {
			return err
		}
	}

	srcPath := r.Cfg.Fullpath()
	if err := checkDBState(srcPath); err != nil {
		return err
	}

	backup := repo.NewBackup(srcPath)
	if files.Exists(backup.Fullpath()) && !Force {
		return fmt.Errorf("%w: %s", repo.ErrBackupAlreadyExists, backup.Name)
	}
	if err := printsBackupInfo(r); err != nil {
		return err
	}
	if !terminal.Confirm("create backup?", "y") {
		return ErrActionAborted
	}
	if err := files.Copy(backup.Src, backup.Fullpath()); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}
	fmt.Println(color.Green("backup created successfully:"), backup.Name)

	return nil
}

// backupPurge purges the excedent backup.
func backupPurge(r *Repo) error {
	var status string

	backups := newBackupList(r.Cfg.BackupPath, r.Cfg.Name)
	purge := backups.TrimElements(r.Cfg.MaxBackups)

	n := purge.Len()
	if n == 0 {
		return repo.ErrBackupNoPurge
	}

	status = presenter.RepoSummary(r)
	status += presenter.BackupsSummary(r)
	f := frame.New(frame.WithColorBorder(color.Gray))

	if n > 0 {
		f.Header(color.BrightRed(n, "backups to delete").Bold().Italic().String())
		purge.ForEach(func(b string) {
			f.Row(presenter.RepoRecordSummary(r, b))
		})

		status += f.String()
	}

	fmt.Println(status)
	nPurgeStr := color.BrightRed("purge").Bold().String()
	if !Force && !terminal.Confirm(fmt.Sprintf("%s %d backup/s?", nPurgeStr, n), "n") {
		return ErrActionAborted
	}

	if err := purge.ForEachErr(func(b string) error {
		fmt.Println("files.Remove(b)", b)
		/* if err := files.Remove(b); err != nil {
			return fmt.Errorf("%w", err)
		} */
		return nil
	}); err != nil {
		return fmt.Errorf("%w", err)
	}

	success := color.BrightGreen("successfully").Italic().Bold()
	fmt.Println("backups purged", success.String())

	return nil
}

// getBackupList retrieves a list of backup files matching the given name in the
// specified directory.
func getBackupList(path, name string) ([]string, error) {
	f, err := files.List(path, name)
	if err != nil {
		return nil, fmt.Errorf("%w: getting files from '%s'", err, path)
	}

	return f, nil
}

func newBackupList(path, name string) *slice.Slice[string] {
	s := slice.New[string]()
	backups, _ := files.List(path, name)

	for _, b := range backups {
		s.Append(&b)
	}

	return s
}

// checkBackupEnabled checks if backups are enabled.
func checkBackupEnabled(n int) error {
	if n <= 0 {
		return repo.ErrBackupDisabled
	}

	return nil
}

// printsBackupInfo prints repository's backup info.
func printsBackupInfo(r *Repo) error {
	s := presenter.RepoSummary(r)
	s += presenter.BackupsSummary(r)
	s += presenter.BackupDetail(r)
	fmt.Print(s)

	return nil
}

// getBackupStatus returns a colored string with the backups status.
func getBackupStatus(n int) string {
	if n <= 0 {
		return color.BrightRed("disabled").String()
	}

	return color.BrightGreen("enabled").String()
}
