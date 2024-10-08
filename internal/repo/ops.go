package repo

import (
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/files"
)

// RecordCount retrieves the maximum ID from the specified table in the
// SQLite database.
func RecordCount(r *SQLiteRepository, t Table) int {
	log.Printf("RecordCount: r: '%s', tableName: '%s'", r.Cfg.Name, t)
	return r.maxID(t)
}

// Tags retrieves and returns a sorted slice of unique tags.
func Tags(r *SQLiteRepository) ([]string, error) {
	t, err := r.ByColumn(r.Cfg.TableMain, "tags")
	if err != nil {
		return nil, err
	}

	var tags []string
	t.ForEach(func(t string) {
		tags = append(tags, strings.Split(t, ",")...)
	})

	slices.Sort(tags)

	return format.Unique(tags), nil
}

// ULRs retrieves and returns a sorted slice of unique urls.
func URLs(r *SQLiteRepository) ([]string, error) {
	urls, err := r.ByColumn(r.Cfg.TableMain, "url")
	if err != nil {
		return nil, err
	}

	u := urls.Items()
	slices.Sort(*u)

	return *u, nil
}

// databasesFromPath returns the list of files from the given path.
func databasesFromPath(p string) (*slice.Slice[string], error) {
	log.Printf("databasesFromPath: path: '%s'", p)
	if !files.Exists(p) {
		return nil, files.ErrPathNotFound
	}

	f, err := files.FindByExtension(p, "db")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return slice.From(f), nil
}

// Databases returns the list of databases.
func Databases(c *SQLiteConfig) (*slice.Slice[*SQLiteRepository], error) {
	p := c.Path
	paths, err := databasesFromPath(p)
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
func Backups(r *SQLiteRepository) (*slice.Slice[string], error) {
	s := filepath.Base(r.Cfg.Fullpath())
	backups, err := databasesFromPath(r.Cfg.Backup.Path)
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
func AddPrefixDate(s string) string {
	now := time.Now().Format(config.DB.BackupDateFormat)
	return fmt.Sprintf("%s_%s", now, s)
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
