package repo

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/files"
)

const commonDBExts = ".sqlite3,.sqlite,.db"

// CountRecords retrieves the maximum ID from the specified table in the
// SQLite database.
func CountRecords(r *SQLiteRepository, t Table) int {
	var n int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", t)
	err := r.DB.QueryRowx(query).Scan(&n)
	if err != nil {
		return 0
	}

	return n
}

// databasesFromPath returns the list of files from the given path.
func databasesFromPath(p string) (*slice.Slice[string], error) {
	log.Printf("databasesFromPath: path: '%s'", p)
	if !files.Exists(p) {
		log.Printf("path does not exist: '%s'", p)
		return nil, files.ErrPathNotFound
	}

	f, err := files.FindByExtList(p, strings.Split(commonDBExts, ",")...)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return slice.New(f...), nil
}

// Find finds a database by name in the given path.
func Find(name, path string) (*SQLiteRepository, error) {
	var found SQLiteRepository
	dbs, err := Databases(path)
	if err != nil {
		return nil, err
	}

	dbs.FilterInPlace(func(db *SQLiteRepository) bool {
		return db.Cfg.Name == files.EnsureExt(name, ".db")
	})
	if dbs.Len() == 0 {
		return nil, fmt.Errorf("'%s' %w", name, ErrDBNotFound)
	}

	found = dbs.Item(0)

	return &found, nil
}

// Databases returns the list of databases.
func Databases(path string) (*slice.Slice[SQLiteRepository], error) {
	// FIX: redo this
	paths, err := databasesFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("'%s' %w", path, err)
	}

	dbs := slice.New[SQLiteRepository]()
	paths.ForEach(func(p string) {
		name := filepath.Base(p)
		path := filepath.Dir(p)
		c := NewSQLiteCfg()
		c.SetPath(path).SetName(name)
		rep, _ := New(c)
		dbs.Push(rep)
	})

	if dbs.Len() == 0 {
		return nil, fmt.Errorf("%w: %s", ErrDBNotFound, path)
	}

	return dbs, nil
}

// CreateBackup creates a new backup.
func CreateBackup(src, destName string, force bool) error {
	log.Printf("CreateBackup: src: %s, dest: %s", src, destName)
	sourcePath := filepath.Dir(src)
	if !files.Exists(sourcePath) {
		return fmt.Errorf("%w: %s", ErrBackupPathNotSet, sourcePath)
	}

	backupPath := filepath.Join(sourcePath, "backup")
	if err := files.MkdirAll(backupPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	destPath := filepath.Join(backupPath, destName)
	if files.Exists(destPath) && !force {
		return fmt.Errorf("%w: %s", ErrBackupAlreadyExists, destName)
	}

	if err := files.Copy(src, destPath); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	return nil
}

// Backups returns a filtered list of backup paths and an error if any.
func Backups(r *SQLiteRepository) (*slice.Slice[SQLiteRepository], error) {
	s := filepath.Base(r.Cfg.Fullpath())
	bksPaths, err := databasesFromPath(r.Cfg.Backup.Path)
	backups := slice.New[SQLiteRepository]()
	if err != nil {
		if errors.Is(err, files.ErrPathNotFound) {
			return backups, fmt.Errorf("%w for '%s'", ErrBackupNotFound, s)
		}

		return backups, err
	}

	bksPaths.FilterInPlace(func(b *string) bool {
		return strings.Contains(*b, s)
	})
	if bksPaths.Len() == 0 {
		return backups, fmt.Errorf("%w: '%s'", ErrBackupNotFound, s)
	}

	err = bksPaths.ForEachErr(func(s string) error {
		c := NewSQLiteCfg()
		c.SetPath(filepath.Dir(s)).SetName(filepath.Base(s))
		r, err := New(c)
		if err != nil {
			return err
		}
		backups.Push(r)

		return nil
	})
	if err != nil {
		return backups, fmt.Errorf("%w", err)
	}

	return backups, nil
}

// AddPrefixDate adds the current date and time to the specified name.
func AddPrefixDate(s, f string) string {
	now := time.Now().Format(f)
	return fmt.Sprintf("%s_%s", now, s)
}
