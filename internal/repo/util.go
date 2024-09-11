package repo

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/haaag/gm/internal/util/files"
	"github.com/haaag/gm/pkg/slice"
)

// IsNonEmptyFile checks if the database is initialized.
func IsNonEmptyFile(f string) bool {
	return files.Size(f) > 0
}

// Exists checks if a database exists.
func Exists(f string) bool {
	return files.Exists(f)
}

// GetRecordCount retrieves the maximum ID from the specified table in the
// SQLite database.
func GetRecordCount(r *SQLiteRepository, tableName string) int {
	log.Printf("GetRecordCount: r: '%s', tableName: '%s'", r.Cfg.Name, tableName)
	return r.getMaxID(tableName)
}

// GetDatabasePaths returns the list of files from the given path.
func GetDatabasePaths(path string) (*slice.Slice[string], error) {
	log.Printf("GetDatabasePaths: path: '%s'", path)
	f, err := files.FindByExtension(path, "db")
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

// AddPrefixDate adds the current date and time to the specified name.
func AddPrefixDate(name string) string {
	now := time.Now().Format(DatabaseBackcupDateFormat)
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

// Info returns the repository info.
func Info(r *SQLiteRepository) string {
	s := Summary(r)
	s += BackupsSummary(r)
	s += BackupDetail(r)

	return s
}

// GetModTime returns the modification time of the specified file.
func GetModTime(s string) string {
	file, err := os.Stat(s)
	if err != nil {
		log.Printf("GetModTime: %v", err)
		return ""
	}

	return file.ModTime().Format(DatabaseBackcupDateFormat)
}
