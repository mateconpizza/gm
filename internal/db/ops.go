package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
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

// TagsCounterFromPath returns a map with tag as key and count as value.
func TagsCounterFromPath(dbPath string) (map[string]int, error) {
	r, err := New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return TagsCounter(r)
}

// DropFromPath drops the database from the given path.
func DropFromPath(dbPath string) error {
	r, err := New(dbPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return Drop(r, context.Background())
}

// CountFavorites returns the number of favorite records.
func CountFavorites(r *SQLiteRepository) int {
	var n int
	query := "SELECT COUNT(*) FROM bookmarks WHERE favorite = 1"
	err := r.DB.QueryRowx(query).Scan(&n)
	if err != nil {
		return 0
	}

	return n
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
//
// locked|unlocked databases.
func Databases(root string) ([]string, error) {
	fs, err := files.FindByExtList(root, ".db", ".enc")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if len(fs) == 0 {
		return nil, ErrDBsNotFound
	}

	return fs, nil
}

// DatabasesUnlocked returns the list of databases.
func DatabasesUnlocked(path string) (*slice.Slice[SQLiteRepository], error) {
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

// DatabasesLocked returns all locked database files.
func DatabasesLocked(root string) ([]string, error) {
	fs, err := files.FindByExtList(root, "enc")
	if err != nil {
		return fs, fmt.Errorf("%w: %q", err, filepath.Base(root))
	}

	return fs, nil
}

// HasURL checks if a record exists in the main table.
func HasURL(bURL string) (*bookmark.Bookmark, error) {
	r, err := New(config.App.DBPath)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	var count int
	q := "SELECT COUNT(*) FROM bookmarks WHERE url = ?"
	if err := r.DB.QueryRowx(q, bURL).Scan(&count); err != nil {
		slog.Error("error getting count", "error", err)
		r.Close()
		return nil, fmt.Errorf("getting count: %w", err)
	}
	if count == 0 {
		return nil, ErrRecordNotFound
	}

	item, err := r.ByURL(bURL)
	if err != nil {
		return nil, fmt.Errorf("ByURL: %w", err)
	}

	return item, nil
}

// newBackup creates a new backup from the given repository.
func newBackup(r *SQLiteRepository) (string, error) {
	if err := files.MkdirAll(config.App.Path.Backup); err != nil {
		return "", fmt.Errorf("%w", err)
	}
	// destDSN -> 20060102-150405_dbName.db
	destDSN := fmt.Sprintf("%s_%s", time.Now().Format(r.Cfg.DateFormat), r.Name())
	destPath := filepath.Join(r.Cfg.BackupDir, destDSN)
	slog.Info("creating SQLite backup",
		"src", r.Cfg.Fullpath(),
		"dest", destPath,
	)
	if files.Exists(destPath) {
		return "", fmt.Errorf("%w: %q", ErrBackupExists, destPath)
	}
	_ = r.DB.MustExec("VACUUM INTO ?", destPath)
	if err := verifySQLiteIntegrity(destPath); err != nil {
		return "", err
	}

	return destDSN, nil
}

// ListDatabaseBackups returns a filtered list of database backups.
func ListDatabaseBackups(dir, dbName string) ([]string, error) {
	// Remove .db|.enc extension for matching
	baseName := files.StripSuffixes(dbName)
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

// isInit returns true if the database is initialized.
func isInit(r *SQLiteRepository) bool {
	allExist := true
	for _, s := range tablesAndSchema() {
		exists, err := r.tableExists(s.name)
		if err != nil {
			slog.Error("checking if table exists", "name", s.name, "error", err)
			return false
		}
		if !exists {
			allExist = false
			slog.Warn("table does not exist", "name", s.name)
		}
	}

	return allExist
}

// IsInitialized checks if the database is initialized.
func IsInitialized(p string) (bool, error) {
	slog.Debug("checking if database is initialized", "path", p)
	allExist := true
	r, err := New(p)
	if err != nil {
		return false, err
	}
	for _, s := range tablesAndSchema() {
		exists, err := r.tableExists(s.name)
		if err != nil {
			slog.Error("checking if table exists", "name", s.name, "error", err)
			return false, err
		}
		if !exists {
			allExist = false
			slog.Warn("table does not exist", "name", s.name)
		}
	}

	return allExist, nil
}

// Drop removes all records database.
func Drop(r *SQLiteRepository, ctx context.Context) error {
	tts := tablesAndSchema()
	tables := make([]Table, 0, len(tts))
	for _, t := range tts {
		tables = append(tables, t.name)
	}

	err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteAll(ctx, tables...); err != nil {
			return fmt.Errorf("%w", err)
		}

		return resetSQLiteSequence(tx, tables...)
	})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return r.Vacuum()
}

func vacuum(r *SQLiteRepository) error {
	slog.Debug("vacuuming database")
	_, err := r.DB.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func resetSQLiteSequence(tx *sqlx.Tx, tables ...Table) error {
	if len(tables) == 0 {
		slog.Warn("no tables provided to reset sqlite sequence")
		return nil
	}

	for _, t := range tables {
		slog.Debug("resetting sqlite sequence", "table", t)
		if _, err := tx.Exec("DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
			return fmt.Errorf("resetting sqlite sequence: %w", err)
		}
	}

	return nil
}
