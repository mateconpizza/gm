package repo

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/haaag/gm/pkg/util/files"
)

const DefBackupMax int = 3

type SQLiteBackup struct {
	Dest string
	Name string // repository basename
	Path string // backup path
	Src  string // repository fullpath
}

// newBackup creates a new backup on the system.
func (b *SQLiteBackup) Create(force bool) error {
	if b.Dest == "" {
		return fmt.Errorf("%w: %s", ErrBackupPathNotSet, b.Name)
	}
	if b.Exists() && !force {
		return fmt.Errorf("%w: %s", ErrBackupAlreadyExists, b.Name)
	}
	if err := files.Copy(b.Src, b.Dest); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	return nil
}

// Remove removes the backup from the system.
func (b *SQLiteBackup) Remove() error {
	fmt.Println("Removing backup: ", b.FullPath())
	/* if err := files.Remove(b.Dest); err != nil {
		return fmt.Errorf("removing file: %w", err)
	} */

	log.Printf("removed backup: '%s'", b.Name)

	return nil
}

// Exists checks if the backup Exists.
func (b *SQLiteBackup) Exists() bool {
	return files.Exists(b.Dest)
}

// SetDestination sets the backup destination.
func (b *SQLiteBackup) SetDestination(s string) {
	b.Dest = s
}

// AddCurrentDate adds a date prefix to the backup name.
func (b *SQLiteBackup) AddCurrentDate() {
	b.Name = files.AddDatePrefix(filepath.Base(b.Src))
}

// FullPath returns the fullpath of the backup.
func (b *SQLiteBackup) FullPath() string {
	return filepath.Join(b.Path, b.Name)
}

// NewBackup creates a new backup.
func NewBackup(src, path string) *SQLiteBackup {
	return &SQLiteBackup{
		Src:  src,
		Path: path,
		Name: filepath.Base(src),
	}
}
