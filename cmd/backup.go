// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"gomarks/pkg/config"
	"gomarks/pkg/format"

	"github.com/spf13/cobra"
)

var (
	bkCreate  bool
	bkList    bool
	bkPurge   bool
	bkRestore bool
)

var (
	ErrBkDisabled      = errors.New("backups are disabled")
	ErrBkNotFound      = errors.New("no backup found")
	ErrBkNoPurge       = errors.New("no backup to purge")
	ErrBkAlreadyExists = errors.New("backup already exists")

	ErrBkCreate  = errors.New("could not create backup")
	ErrBkRemove  = errors.New("could not remove backup")
	ErrBkRestore = errors.New("could not restore backup")
	ErrBkStatus  = errors.New("could not get backup status")
)

// getBkStateColored returns a colored string with the backups status
func getBkStateColored() string {
	if config.App.BackupMax <= 0 {
		return format.Text("disabled").Red().String()
	}
	return format.Text("enabled").Green().String()
}

// getBackups returns a list of backups
func getBackups(name string) ([]string, error) {
	q := config.App.Path.Backup + "/*" + name
	f, err := filepath.Glob(q)
	if err != nil {
		return nil, fmt.Errorf("%w: getting with: %s", err, q)
	}
	return f, nil
}

// getBackupsToPurge returns a list of backups filtered by n
func getBackupsToPurge(files []string, n int) []string {
	var filtered []string
	if len(files) > n {
		filtered = files[:len(files)-n]
	}
	return filtered
}

// genBackupName generates a backup name
func genBackupName(s string) string {
	now := time.Now().Format("2006-01-02_15-04")
	return fmt.Sprintf("%s_%s", now, s)
}

// checkBackupEnabled checks if backups are enabled
func checkBackupEnabled() error {
	if config.App.BackupMax <= 0 {
		return fmt.Errorf("%w", ErrBkDisabled)
	}
	return nil
}

// handleBackupCreate creates a backup
func handleBackupCreate(args []string) error {
	srcName := getDBNameFromArgs(args)
	srcPath := filepath.Join(config.App.Path.Databases, srcName)
	if err := checkDBState(srcPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	destName := genBackupName(srcName)
	destPath := filepath.Join(config.App.Path.Backup, destName)
	if fileExists(destPath) {
		return fmt.Errorf("%w: %s", ErrBkAlreadyExists, destPath)
	}

	s := backupDetail(srcName)
	s += "\n" + backupList(srcName)
	fmt.Println(s)

	if err := checkBackupEnabled(); err != nil {
		return err
	}

	if !confirm("create backup?", "Y") {
		return nil
	}

	if err := fileCopy(srcPath, destPath); err != nil {
		return err
	}

	fmt.Println(format.Text("backup created successfully:").Green(), destName)
	return nil
}

func backupList(s string) string {
	files, err := getBackups(s)
	if err != nil || len(files) == 0 {
		return ""
	}

	bks := make([]string, 0, len(files))
	for _, f := range files {
		f := filepath.Base(f)
		bks = append(bks, format.BulletLine(f, ""))
	}

	t := format.Text("backup/s").Purple().Bold().String()
	return format.HeaderWithSection(t, bks)
}

func backupDetail(s string) string {
	n := config.App.BackupMax
	files, err := getBackups(s)
	if err != nil {
		return ""
	}

	f := format.Text(strconv.Itoa(len(files))).Green().String()
	if len(files) > n {
		f = format.Text(strconv.Itoa(len(files))).Red().String()
	}

	a := format.Text(strconv.Itoa(n)).Green().String()
	return format.HeaderWithSection(format.Text("database").Yellow().Bold().String(), []string{
		format.BulletLine("name:", format.Text(s).Blue().String()),
		format.BulletLine("status: ", getBkStateColored()),
		format.BulletLine("max:", fmt.Sprintf("%s backups allowed", a)),
		format.BulletLine("backup/s:", fmt.Sprintf("%s backups found", f)),
	})
}

func backupPurge(args []string) error {
	name := getDBNameFromArgs(args)
	bks, err := getBackups(name)
	if err != nil {
		return err
	}

	files := getBackupsToPurge(bks, config.App.BackupMax)
	p := make([]string, 0, len(files))
	for _, f := range files {
		p = append(p, format.BulletLine(filepath.Base(f), ""))
	}

	del := format.Text(strconv.Itoa(len(files))).Red().Bold().String()
	s := backupDetail(name)
	s += format.BulletLine("purge:", fmt.Sprintf("%s backups to delete", del))
	if len(p) > 0 {
		s += format.HeaderWithSection(format.Text("\nbackup/s to purge:").Red().Bold().String(), p)
	}
	fmt.Println(s)

	if len(files) == 0 {
		return fmt.Errorf("%w", ErrBkNoPurge)
	}

	if !forceFlag && !confirm(fmt.Sprintf("purge %s backups?", del), "N") {
		return nil
	}

	for _, s := range files {
		if err := rmFile(s); err != nil {
			return fmt.Errorf("%w", err)
		}
	}
	fmt.Println(format.Text("backups purged successfully").Green())
	return nil
}

func backupRestore(args []string) error {
	s := getDBNameFromArgs(args)
	b, err := getBackups(s)
	if err != nil {
		return err
	}

	for i := 0; i < len(b); i++ {
		f := filepath.Base(b[i])
		fmt.Println(i, format.Text(f).Yellow())
	}

	return nil
}

var backupCmd = &cobra.Command{
	Use:   "bk",
	Short: "backup management",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		maxBackups, err := strconv.Atoi(config.GetEnv(config.App.Env.BackupMax, "3"))
		if err != nil {
			return fmt.Errorf("loading maxBackups: %w", err)
		}
		config.App.SetBackupMax(maxBackups)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := map[bool]func(args []string) error{
			bkCreate:  handleBackupCreate,
			bkList:    handleDBInfo,
			bkPurge:   backupPurge,
			bkRestore: backupRestore,
		}
		if handler, ok := flags[true]; ok {
			if err := handler(args); err != nil {
				return fmt.Errorf("%w", err)
			}
		}
		return nil
	},
}

func init() {
	backupCmd.Flags().BoolVarP(&bkCreate, "create", "c", false, "create backup")
	backupCmd.Flags().BoolVarP(&bkList, "list", "l", false, "list backups")
	backupCmd.Flags().BoolVarP(&bkPurge, "purge", "p", false, "remove backup")
	backupCmd.Flags().BoolVarP(&bkRestore, "restore", "R", false, "restore backup")
	rootCmd.AddCommand(backupCmd)
}
