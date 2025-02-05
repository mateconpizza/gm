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

	found := dbs.Item(0)

	return &found, nil
}

// Databases returns the list of databases.
func Databases(path string) (*slice.Slice[SQLiteRepository], error) {
	paths, err := databasesFromPath(path)
	if err != nil {
		if errors.Is(err, files.ErrPathNotFound) {
			return nil, ErrBackupNotFound
		}

		return nil, fmt.Errorf("'%s' %w", path, err)
	}
	if paths.Len() == 0 {
		return nil, ErrBackupNotFound
	}

	dbs := slice.New[SQLiteRepository]()
	paths.ForEach(func(p string) {
		rep, _ := New(NewSQLiteCfg(p))
		dbs.Push(rep)
	})
	if dbs.Len() == 0 {
		return nil, fmt.Errorf("%w: %s", ErrDBNotFound, path)
	}

	return dbs, nil
}

// NewBackup creates a new backup from the given repository.
func NewBackup(r *SQLiteRepository) (string, error) {
	bk := r.Cfg.Backup
	if err := files.MkdirAll(bk.Path); err != nil {
		return "", fmt.Errorf("%w", err)
	}
	destDSN := AddPrefixDate(r.Cfg.Name, bk.DateFormat)
	_ = r.DB.MustExec("VACUUM INTO ?", filepath.Join(bk.Path, destDSN))

	return destDSN, nil
}

// getBackups returns a filtered list of backups in a given path.
func getBackups(path string) []string {
	paths, err := databasesFromPath(path)
	if err != nil {
		return []string{}
	}
	name := filepath.Base(path)
	paths.FilterInPlace(func(s *string) bool {
		return strings.Contains(*s, name)
	})

	return *paths.Items()
}

// Backups returns a filtered list of backup paths and an error if any.
func Backups(r *SQLiteRepository) (*slice.Slice[SQLiteRepository], error) {
	s := filepath.Base(r.Cfg.Fullpath())
	paths, err := databasesFromPath(r.Cfg.Backup.Path)
	if err != nil {
		if errors.Is(err, files.ErrPathNotFound) {
			return nil, ErrBackupNotFound
		}

		return nil, err
	}
	// filter by current repo name
	paths.FilterInPlace(func(b *string) bool {
		return strings.Contains(*b, s)
	})
	if paths.Len() == 0 {
		return nil, fmt.Errorf("%w: '%s'", ErrBackupNotFound, s)
	}
	backups := slice.New[SQLiteRepository]()
	if err = paths.ForEachErr(func(s string) error {
		r, err := New(NewSQLiteCfg(s))
		if err != nil {
			return err
		}
		backups.Push(r)

		return nil
	}); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return backups, nil
}

// AddPrefixDate adds the current date and time to the specified name.
func AddPrefixDate(s, f string) string {
	now := time.Now().Format(f)
	return fmt.Sprintf("%s_%s", now, s)
}
