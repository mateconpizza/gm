package repo

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/haaag/gm/internal/util/files"
	"github.com/haaag/gm/pkg/slice"
)

var (
	ErrBackupAlreadyExists = errors.New("backup already exists")
	ErrBackupCreate        = errors.New("could not create backup")
	ErrBackupDisabled      = errors.New("backups are disabled")
	ErrBackupNoPurge       = errors.New("no backup to purge")
	ErrBackupNotFound      = errors.New("no backup found")
	ErrBackupRemove        = errors.New("could not remove backup")
	ErrBackupStatus        = errors.New("could not get backup status")
	ErrBackupPathNotSet    = errors.New("backup path not set")
)

// CreateBackup creates a new backup.
func CreateBackup(src, destName string, force bool) error {
	log.Printf("CreateBackup: src: %s, dest: %s", src, destName)
	path := filepath.Dir(src)
	if !Exists(path) {
		return fmt.Errorf("%w: %s", ErrBackupPathNotSet, path)
	}

	destPath := filepath.Join(path, "backup", destName)
	if Exists(destPath) && !force {
		return fmt.Errorf("%w: %s", ErrBackupAlreadyExists, destName)
	}

	if err := files.Copy(src, destPath); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	return nil
}

func GetBackups(r *SQLiteRepository) (*slice.Slice[string], error) {
	s := filepath.Base(r.Cfg.Fullpath())
	backups, err := GetDatabasePaths(r.Cfg.BackupPath)
	backups.Filter(func(b string) bool {
		return strings.Contains(b, s)
	})

	if err != nil {
		return backups, err
	}

	if backups.Len() == 0 {
		return backups, fmt.Errorf("%w: '%s'", ErrBackupNotFound, s)
	}

	return backups, nil
}
