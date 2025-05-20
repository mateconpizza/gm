package repo

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/files"
)

// CountMainRecords returns the number of records in the main table.
func CountMainRecords(r *SQLiteRepository) int {
	slog.Debug("count main records", "database", r.Name())
	return countRecords(r, schemaMain.name)
}

// CountTagsRecords returns the number of records in the tags table.
func CountTagsRecords(r *SQLiteRepository) int {
	slog.Debug("count tags records", "database", r.Name())
	return countRecords(r, schemaTags.name)
}

// count counts the number of rows in the specified table.
func countRecords(r *SQLiteRepository, t Table) int {
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
	slog.Debug("databases", "path", p)
	if !files.Exists(p) {
		return nil, files.ErrPathNotFound
	}

	f, err := files.FindByExtList(p, ".db")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return slice.New(f...), nil
}

// Databases returns the list of databases.
func Databases(path string) (*slice.Slice[SQLiteRepository], error) {
	paths, err := databasesFromPath(path)
	if err != nil {
		if errors.Is(err, files.ErrPathNotFound) {
			return nil, ErrDBNotFound
		}

		return nil, fmt.Errorf("%q %w", path, err)
	}
	dbs := slice.New[SQLiteRepository]()
	err = paths.ForEachErr(func(p string) error {
		rep, _ := New(p)
		dbs.Push(rep)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return dbs, nil
}

// DatabasesEncrypted returns all encrypted database files.
func DatabasesEncrypted(root string) ([]string, error) {
	fs, err := files.FindByExtList(root, "enc")
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}

	return fs, nil
}

// newBackup creates a new backup from the given repository.
func newBackup(r *SQLiteRepository) (string, error) {
	if err := files.MkdirAll(r.Cfg.BackupDir); err != nil {
		return "", fmt.Errorf("%w", err)
	}
	// destDSN -> 20060102-150405_dbName.db
	destDSN := fmt.Sprintf("%s_%s", time.Now().Format(r.Cfg.DateFormat), r.Name())
	destPath := filepath.Join(r.Cfg.BackupDir, destDSN)
	slog.Info("creating SQLite backup",
		"src", r.Cfg.Fullpath(),
		"dest", destPath,
	)
	_ = r.DB.MustExec("VACUUM INTO ?", destPath)
	if err := verifySQLiteIntegrity(destPath); err != nil {
		return "", err
	}

	return destDSN, nil
}

// listDatabaseBackups returns a filtered list of database backups.
func listDatabaseBackups(dir, dbName string) ([]string, error) {
	// Remove .db extension for matching
	baseName := strings.TrimSuffix(dbName, ".db")
	entries, err := filepath.Glob(filepath.Join(dir, "*_"+baseName+".db*"))
	if err != nil {
		return nil, fmt.Errorf("listing backups: %w", err)
	}

	return entries, nil
}

// Backups returns a filtered list of backup paths and an error if any.
func Backups(r *SQLiteRepository) (*slice.Slice[SQLiteRepository], error) {
	backups := slice.New[SQLiteRepository]()
	bkPaths, err := r.BackupsList()
	if err != nil {
		return backups, err
	}
	for _, p := range bkPaths {
		backup, err := NewFromBackup(p)
		if err != nil {
			return backups, err
		}

		backups.Push(backup)
	}

	return backups, nil
}

// verifySQLiteIntegrity checks the integrity of the SQLite database.
func verifySQLiteIntegrity(path string) error {
	slog.Debug("verifying SQLite integrity", "path", path)

	db, err := openDatabase(path)
	if err != nil {
		return fmt.Errorf("no se pudo abrir backup: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("error closing db", "error", err)
		}
	}()

	var result string
	row := db.QueryRow("PRAGMA integrity_check;")
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("%w: %w", ErrDBCorrupted, err)
	}

	if result != "ok" {
		return fmt.Errorf("%w: integrity check: %q", ErrDBCorrupted, result)
	}

	slog.Debug("SQLite integrity verified", "result", result)

	return nil
}
