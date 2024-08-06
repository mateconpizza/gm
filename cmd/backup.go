package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util"
	"github.com/haaag/gm/pkg/util/files"
)

// TODO)):
// - [ ] implement the `slice` struct

var (
	bkCreate  bool
	bkList    bool
	bkPurge   bool
	bkRestore bool
)

// backupCmd backup management.
var backupCmd = &cobra.Command{
	Use:   "bk",
	Short: "backup management",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}

		flags := map[bool]func(r *Repo) error{
			bkCreate:  handleBackupCreate,
			bkPurge:   handleBackupPurge,
			bkList:    printsBackupInfo,
			bkRestore: handleBackupRestore,
		}

		if handler, run := flags[true]; run {
			return handler(r)
		}

		return printsBackupInfo(r)
	},
}

func init() {
	backupCmd.Flags().BoolVarP(&bkCreate, "create", "c", false, "create backup")
	backupCmd.Flags().BoolVarP(&bkList, "list", "l", false, "list backups")
	backupCmd.Flags().BoolVarP(&bkPurge, "purge", "P", false, "remove backup")
	backupCmd.Flags().BoolVarP(&bkRestore, "restore", "R", false, "restore backup")
	rootCmd.AddCommand(backupCmd)
}

// handleBackupCreate creates a backup of the specified repository if
// conditions are met, including confirmation and backup limits.
func handleBackupCreate(r *Repo) error {
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
	if err := checkBackupEnabled(r.Cfg.MaxBackups); err != nil {
		return err
	}
	if !Force && !terminal.Confirm("create backup?", "y") {
		return ErrActionAborted
	}
	if err := files.Copy(backup.Src, backup.Fullpath()); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}
	fmt.Println(color.Green("backup created successfully:"), backup.Name)

	return nil
}

// handleBackupPurge purges the excedent backup.
func handleBackupPurge(r *Repo) error {
	var (
		backupList []string
		nPurgeStr  string
		purgeList  []string
		status     string
		toPurge    []string
	)

	backupList, _ = getBackups(r.Cfg.BackupPath, r.Cfg.Name)
	toPurge = util.TrimElements(backupList, r.Cfg.MaxBackups)
	n := len(toPurge)
	if n == 0 {
		return repo.ErrBackupNoPurge
	}

	purgeList = getDBsBasename(toPurge)
	status = backupDetail(r)
	nPurgeStr = color.Red(strconv.Itoa(n)).Bold().String()
	if n > 0 {
		status += format.BulletLine("purge:", nPurgeStr+" backups to delete")
		status += format.HeaderWithSection(
			color.Red("\nbackup/s to purge:").Bold().String(),
			purgeList,
		)
	}

	fmt.Println(status)
	if !Force && !terminal.Confirm(fmt.Sprintf("purge %s backups?", nPurgeStr), "n") {
		return ErrActionAborted
	}

	for _, s := range toPurge {
		if err := files.Remove(s); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	fmt.Println(color.Green("backups purged successfully"))

	return nil
}

// handleBackupRestore.
func handleBackupRestore(_ *Repo) error {
	fmt.Println("Backup restore...")
	return nil
}

// backupInfo returns a formatted string with information about backups for a
// given repository, including a message if no backups are found.
func backupInfo(r *Repo) string {
	t := color.Purple("backup/s").Bold().String()
	bks, _ := getBackups(r.Cfg.BackupPath, r.Cfg.Name)
	if len(bks) == 0 {
		return format.HeaderWithSection(t, []string{format.BulletLine("no backups found", "")})
	}

	bs := getDBsBasename(bks)

	return format.HeaderWithSection(t, bs)
}

// getBackups retrieves a list of backup files matching the given name in the
// specified directory.
func getBackups(path, name string) ([]string, error) {
	f, err := files.List(path, name)
	if err != nil {
		return nil, fmt.Errorf("%w: getting files from '%s'", err, path)
	}

	return f, nil
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
	var sb strings.Builder
	sb.WriteString(backupDetail(r) + "\n")
	sb.WriteString(backupInfo(r))
	fmt.Println(sb.String())

	return nil
}

// getBkStateColored returns a colored string with the backups status.
func getBkStateColored(n int) string {
	if n <= 0 {
		return color.Red("disabled").String()
	}

	return color.Green("enabled").String()
}

// backupDetail returns a detailed list of backups.
func backupDetail(r *Repo) string {
	var (
		bks, _     = getBackups(r.Cfg.BackupPath, r.Cfg.Name)
		found      = len(bks)
		foundColor string
		n          = r.Cfg.MaxBackups
	)

	if found > n {
		foundColor = color.Red(strconv.Itoa(found)).Bold().String()
	} else {
		foundColor = color.Green(strconv.Itoa(found)).Bold().String()
	}

	allowed := color.Green(strconv.Itoa(n)).String()
	header := color.Yellow("database").Bold().String()

	return format.HeaderWithSection(header, []string{
		format.BulletLine("name:", color.Blue(r.Cfg.Name).String()),
		format.BulletLine("status: ", getBkStateColored(n)),
		format.BulletLine("max:", allowed+" backups allowed"),
		format.BulletLine("backup/s:", foundColor+" backups found"),
	})
}
