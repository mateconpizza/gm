package repo

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/util/files"
)

// GetRecordCount retrieves the maximum ID from the specified table in the
// SQLite database.
func GetRecordCount(r *SQLiteRepository, tableName string) int {
	log.Printf("GetRecordCount: r: '%s', tableName: '%s'", r.Cfg.Name, tableName)
	return r.getMaxID(tableName)
}

// GetDatabasePaths returns the list of files from the given path.
func GetDatabasePaths(p string) (*slice.Slice[string], error) {
	log.Printf("GetDatabasePaths: path: '%s'", p)
	if !files.Exists(p) {
		return nil, files.ErrPathNotFound
	}

	f, err := files.FindByExtension(p, "db")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return slice.From(f), nil
}

// GetDatabases returns the list of databases.
func GetDatabases(c *SQLiteConfig) (*slice.Slice[*SQLiteRepository], error) {
	p := c.Path
	paths, err := GetDatabasePaths(p)
	if err != nil {
		return nil, err
	}

	dbs := slice.New[*SQLiteRepository]()
	paths.ForEach(func(p string) {
		name := filepath.Base(p)
		path := filepath.Dir(p)

		c := NewSQLiteCfg(path)
		c.SetName(name)

		rep, _ := New(c)
		dbs.Append(&rep)
	})

	return dbs, nil
}

// CreateBackup creates a new backup.
func CreateBackup(src, destName string, force bool) error {
	log.Printf("CreateBackup: src: %s, dest: %s", src, destName)
	path := filepath.Dir(src)
	if !files.Exists(path) {
		return fmt.Errorf("%w: %s", ErrBackupPathNotSet, path)
	}

	backupPath := filepath.Join(path, "backup")
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

func GetBackups(r *SQLiteRepository) (*slice.Slice[string], error) {
	s := filepath.Base(r.Cfg.Fullpath())
	backups, err := GetDatabasePaths(r.Cfg.Backup.Path)
	if err != nil {
		return nil, err
	}

	backups.Filter(func(b string) bool {
		return strings.Contains(b, s)
	})

	if backups.Len() == 0 {
		return backups, fmt.Errorf("%w: '%s'", ErrBackupNotFound, s)
	}

	return backups, nil
}

// AddPrefixDate adds the current date and time to the specified name.
func AddPrefixDate(name string) string {
	now := time.Now().Format(config.DB.BackupDateFormat)
	return fmt.Sprintf("%s_%s", now, name)
}

// Remove removes a repository from the system.
func Remove(f string) error {
	name := filepath.Base(f)
	log.Printf("Remove: repository: '%s'", name)
	if err := files.Remove(f); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}

	log.Printf("Remove: removed repository: '%s'", f)

	return nil
}
