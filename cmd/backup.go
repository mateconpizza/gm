package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util"
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

func handleBackupCreate(r *Repo) error {
	srcPath := r.Cfg.Fullpath()
	if err := checkDBState(srcPath); err != nil {
		return err
	}

	backup := repo.NewBackup(srcPath)
	if util.FileExists(backup.Fullpath()) && !Force {
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
	if err := util.CopyFile(backup.Src, backup.Fullpath()); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}
	fmt.Println(C("backup created successfully:").Green(), backup.Name)

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
	nPurgeStr = C(strconv.Itoa(n)).Red().Bold().String()
	if n > 0 {
		status += format.BulletLine("purge:", nPurgeStr+" backups to delete")
		status += format.HeaderWithSection(
			C("\nbackup/s to purge:").Red().Bold().String(),
			purgeList,
		)
	}

	fmt.Println(status)
	if !Force && !terminal.Confirm(fmt.Sprintf("purge %s backups?", nPurgeStr), "n") {
		return ErrActionAborted
	}

	for _, s := range toPurge {
		if err := util.RmFile(s); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	fmt.Println(C("backups purged successfully").Green())

	return nil
}

// handleBackupRestore.
func handleBackupRestore(_ *Repo) error {
	fmt.Println("Backup restore...")
	return nil
}

// backupInfo.
func backupInfo(r *Repo) string {
	t := C("backup/s").Purple().Bold().String()
	bks, _ := getBackups(r.Cfg.BackupPath, r.Cfg.Name)
	if len(bks) == 0 {
		return format.HeaderWithSection(t, []string{format.BulletLine("no backups found", "")})
	}

	bs := getDBsBasename(bks)

	return format.HeaderWithSection(t, bs)
}

func getBackups(path, name string) ([]string, error) {
	f, err := util.Files(path, name)
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
		return C("disabled").Red().String()
	}

	return C("enabled").Green().String()
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
		foundColor = C(strconv.Itoa(found)).Red().String()
	} else {
		foundColor = C(strconv.Itoa(found)).Green().String()
	}

	a := C(strconv.Itoa(n)).Green().String()

	return format.HeaderWithSection(C("database").Yellow().Bold().String(), []string{
		format.BulletLine("name:", C(r.Cfg.Name).Blue().String()),
		format.BulletLine("status: ", getBkStateColored(n)),
		format.BulletLine("max:", a+" backups allowed"),
		format.BulletLine("backup/s:", foundColor+" backups found"),
	})
}
