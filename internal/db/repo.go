// Package repo provides the model of the configuration for a database.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/mateconpizza/gm/internal/sys/files"
)

type Table string

const (
	// Default date format for timestamps.
	defaultDateFormat = "20060102-150405"
)

// SQLiteRepository implements the Repository interface.
type SQLiteRepository struct {
	DB        *sqlx.DB   `json:"-"`
	Cfg       *SQLiteCfg `json:"db"`
	closeOnce sync.Once
}

// Name returns the name of the SQLite database.
func (r *SQLiteRepository) Name() string {
	return r.Cfg.Name
}

// Close closes the SQLite database connection and logs any errors encountered.
func (r *SQLiteRepository) Close() {
	s := r.Name()
	r.closeOnce.Do(func() {
		if err := r.DB.Close(); err != nil {
			slog.Error("closing database", "name", s, "error", err)
		} else {
			slog.Debug("database closed", "name", s)
		}
	})
}

// ListBackups returns a list of available backup files.
func (r *SQLiteRepository) ListBackups() ([]string, error) {
	return ListBackups(r.Cfg.BackupDir, r.Cfg.Name)
}

// Backup creates a backup of the SQLite database and returns the path to the
// backup filepath.
func (r *SQLiteRepository) Backup() (string, error) {
	return newBackup(r)
}

// IsInitialized returns true if the database is initialized.
func (r *SQLiteRepository) IsInitialized() bool {
	return isInit(r)
}

// newSQLiteRepository returns a new SQLiteRepository.
func newSQLiteRepository(db *sqlx.DB, cfg *SQLiteCfg) *SQLiteRepository {
	return &SQLiteRepository{
		DB:  db,
		Cfg: cfg,
	}
}

// New returns a new SQLiteRepository from an existing database path.
func New(p string) (*SQLiteRepository, error) {
	return newRepository(p, func(path string) error {
		slog.Debug("new repo: checking if database exists", "path", path)
		if !files.Exists(path) {
			return fmt.Errorf("%w: %q", ErrDBNotFound, path)
		}

		return nil
	})
}

// Init initializes a new SQLiteRepository at the provided path.
func Init(p string) (*SQLiteRepository, error) {
	return newRepository(p, func(path string) error {
		slog.Debug("init repo: checking if database exists", "path", path)
		if files.Exists(path) {
			return fmt.Errorf("%w: %q", ErrDBExists, path)
		}

		return nil
	})
}

// newRepository returns a new SQLiteRepository from the provided path.
func newRepository(p string, validate func(string) error) (*SQLiteRepository, error) {
	if p == "" {
		return nil, files.ErrPathEmpty
	}
	if err := validate(p); err != nil {
		return nil, err
	}
	c, err := newSQLiteCfg(p)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	db, err := openDatabase(p)
	if err != nil {
		slog.Error("NewRepo", "error", err, "path", p)
		return nil, err
	}

	return newSQLiteRepository(db, c), nil
}

// NewFromBackup creates a SQLiteRepository from a backup file.
func NewFromBackup(backupPath string) (*SQLiteRepository, error) {
	if backupPath == "" {
		return nil, files.ErrPathNotFound
	}
	backupFile := filepath.Base(backupPath)
	parentDir := filepath.Dir(backupPath)

	// check if we're already in a backup directory
	if filepath.Base(parentDir) == "backup" {
		parentDir = filepath.Dir(filepath.Dir(backupPath))
	} else {
		parentDir = filepath.Dir(backupPath)
	}
	backupDir := filepath.Join(parentDir, "backup")
	cfg := &SQLiteCfg{
		Path:      backupDir,
		Name:      backupFile,
		BackupDir: backupDir,
	}

	slog.Debug("reading backup", "name", backupFile)
	db, err := openDatabase(backupPath)
	if err != nil {
		return nil, fmt.Errorf("opening backup database: %w", err)
	}

	return &SQLiteRepository{
		DB:  db,
		Cfg: cfg,
	}, nil
}

// openDatabase opens a SQLite database at the specified path and verifies
// the connection, returning the database handle or an error.
func openDatabase(s string) (*sqlx.DB, error) {
	slog.Debug("opening database", "path", s)
	db, err := sqlx.Open("sqlite3", s)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("%w: on ping context", err)
	}

	return db, nil
}

// SQLiteCfg represents the configuration for a SQLite database.
type SQLiteCfg struct {
	Name        string   `json:"name"`         // Name of the SQLite database
	Path        string   `json:"path"`         // Path to the SQLite database
	BackupDir   string   `json:"backup_path"`  // Backup path
	BackupFiles []string `json:"backup_files"` // Backup files
	DateFormat  string   `json:"date_format"`  // Date format
}

// Fullpath returns the full path to the SQLite database.
func (c *SQLiteCfg) Fullpath() string {
	return filepath.Join(c.Path, c.Name)
}

// Exists returns true if the SQLite database exists.
func (c *SQLiteCfg) Exists() bool {
	return files.Exists(c.Fullpath())
}

// newSQLiteCfg returns the default settings for the database.
func newSQLiteCfg(p string) (*SQLiteCfg, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %q: %w", p, err)
	}
	baseDir := filepath.Dir(abs)

	return &SQLiteCfg{
		Path:       baseDir,
		Name:       files.EnsureSuffix(filepath.Base(abs), ".db"),
		BackupDir:  filepath.Join(baseDir, "backup"),
		DateFormat: defaultDateFormat,
	}, nil
}
