package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util"
)

// TODO)):
// - [ ] implement the `slice` struct

const backupDateFormat = "2006-01-02_15-04"

var (
	bkCreate  bool
	bkList    bool
	bkPurge   bool
	bkRestore bool
)

var (
	ErrBackupDisabled      = errors.New("backups are disabled")
	ErrBackupNotFound      = errors.New("no backup found")
	ErrBackupNoPurge       = errors.New("no backup to purge")
	ErrBackupAlreadyExists = errors.New("backup already exists")
	ErrBackupCreate        = errors.New("could not create backup")
	ErrBackupRemove        = errors.New("could not remove backup")
	ErrBackupRestore       = errors.New("could not restore backup")
	ErrBackupStatus        = errors.New("could not get backup status")
)

// backupCmd backup management
var backupCmd = &cobra.Command{
	Use:   "bk",
	Short: "backup management",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		if err := loadBackups(r); err != nil {
			return err
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

// handleBackupCreate creates a backup
func handleBackupCreate(r *Repo) error {
	var (
		srcName, srcPath   string
		destName, destPath string
	)

	srcName = r.Cfg.GetName()
	srcPath = filepath.Join(r.Cfg.GetHome(), r.Cfg.GetName())
	if err := checkDBState(srcPath); err != nil {
		return err
	}

	destName = generateBackupName(srcName)
	destPath = filepath.Join(r.Cfg.Backup.GetHome(), destName)
	if util.FileExists(destPath) && !Force {
		return fmt.Errorf("%w: %s", ErrBackupAlreadyExists, destPath)
	}

	if err := printsBackupInfo(r); err != nil {
		return err
	}
	if err := checkBackupEnabled(r.Cfg.Backup.GetMax()); err != nil {
		return err
	}

	if !Force && !terminal.Confirm("create backup?", "y") {
		return ErrActionAborted
	}

	if err := util.CopyFile(srcPath, destPath); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	fmt.Println(C("backup created successfully:").Green(), destName)
	return nil
}

// handleBackupPurge
func handleBackupPurge(r *Repo) error {
	var (
		backupList []string
		maxBackups int
		nPurge     int
		nPurgeStr  string
		purgeList  []string
		status     string
		toPurge    []string
	)

	maxBackups = r.Cfg.Backup.GetMax()
	backupList = r.Cfg.Backup.List()
	toPurge = getBackupsToPurge(backupList, maxBackups)
	nPurge = len(toPurge)
	if nPurge == 0 {
		return ErrBackupNoPurge
	}

	purgeList = getDBsBasename(toPurge)
	status = backupDetail(r)
	nPurgeStr = C(strconv.Itoa(nPurge)).Red().Bold().String()
	if nPurge > 0 {
		status += format.BulletLine("purge:", fmt.Sprintf("%s backups to delete", nPurgeStr))
		status += format.HeaderWithSection(C("\nbackup/s to purge:").Red().Bold().String(), purgeList)
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

// handleBackupRestore
func handleBackupRestore(_ *Repo) error {
	fmt.Println("Backup restore...")
	return nil
}

// backupInfo
func backupInfo(r *Repo) string {
	t := C("backup/s").Purple().Bold().String()
	if len(r.Cfg.Backup.List()) == 0 {
		return format.HeaderWithSection(t, []string{format.BulletLine("no backups found", "")})
	}

	bs := getDBsBasename(r.Cfg.Backup.List())
	return format.HeaderWithSection(t, bs)
}

// loadBackups returns a list of backups
func loadBackups(r *Repo) error {
	f, err := util.Files(r.Cfg.Backup.GetHome(), r.Cfg.GetName())
	if err != nil {
		return fmt.Errorf("loading backups: %w", err)
	}
	r.Cfg.Backup.Load(f)
	return nil
}

// getBackupsToPurge returns a list of backups filtered by n
func getBackupsToPurge(files []string, n int) []string {
	var filtered []string
	if len(files) > n {
		filtered = files[:len(files)-n]
	}
	return filtered
}

// generateBackupName generates a backup name
func generateBackupName(s string) string {
	now := time.Now().Format(backupDateFormat)
	return fmt.Sprintf("%s_%s", now, s)
}

// checkBackupEnabled checks if backups are enabled
func checkBackupEnabled(n int) error {
	if n <= 0 {
		return ErrBackupDisabled
	}
	return nil
}

// printsBackupInfo prints repository's backup info
func printsBackupInfo(r *Repo) error {
	// FIX: why return a error? remove it
	var sb strings.Builder
	sb.WriteString(backupDetail(r) + "\n")
	sb.WriteString(backupInfo(r))
	fmt.Println(sb.String())
	return nil
}

// getBkStateColored returns a colored string with the backups status
func getBkStateColored(n int) string {
	if n <= 0 {
		return C("disabled").Red().String()
	}
	return C("enabled").Green().String()
}

// backupDetail returns a detailed list of backups
func backupDetail(r *Repo) string {
	var (
		n          int
		found      int
		foundColor string
	)

	n = r.Cfg.Backup.GetMax()
	found = len(r.Cfg.Backup.List())
	if found > n {
		foundColor = C(strconv.Itoa(found)).Red().String()
	} else {
		foundColor = C(strconv.Itoa(found)).Green().String()
	}

	a := C(strconv.Itoa(n)).Green().String()
	return format.HeaderWithSection(C("database").Yellow().Bold().String(), []string{
		format.BulletLine("name:", C(r.Cfg.GetName()).Blue().String()),
		format.BulletLine("status: ", getBkStateColored(n)),
		format.BulletLine("max:", fmt.Sprintf("%s backups allowed", a)),
		format.BulletLine("backup/s:", fmt.Sprintf("%s backups found", foundColor)),
	})
}
