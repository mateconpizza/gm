// Package repo provides the model of the configuration for a database.
package repo

import (
	"path/filepath"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/sys/files"
)

// SQLiteConfig represents the configuration for a SQLite database.
type SQLiteConfig struct {
	Name         string       `json:"name"`
	Path         string       `json:"path"`
	TableMain    string       `json:"table_main"`
	TableDeleted string       `json:"table_deleted"`
	Backup       SQLiteBackup `json:"backup"`
	MaxBytesSize int64        `json:"max_bytes_size"`
	MaxBackups   int          `json:"max_backups_allowed"`
}

type SQLiteBackup struct {
	Path    string   `json:"path"`
	Files   []string `json:"files"`
	Limit   int      `json:"limit"`
	Enabled bool     `json:"enabled"`
}

func (b *SQLiteBackup) SetLimit(n int) {
	b.Limit = n
	b.Enabled = n > 0
}

func newSQLiteBackup(p string) *SQLiteBackup {
	return &SQLiteBackup{
		Path:    filepath.Join(p, "backup"),
		Files:   []string{},
		Enabled: false,
		Limit:   0,
	}
}

func (c *SQLiteConfig) Fullpath() string {
	return filepath.Join(c.Path, c.Name)
}

func (c *SQLiteConfig) SetPath(path string) *SQLiteConfig {
	c.Path = path
	return c
}

func (c *SQLiteConfig) SetName(name string) *SQLiteConfig {
	c.Name = files.EnsureExtension(name, ".db")
	return c
}

func (c *SQLiteConfig) Exists() error {
	if !files.Exists(c.Fullpath()) {
		return ErrDBNotFound
	}

	return nil
}

// NewSQLiteCfg returns the default settings for the database.
func NewSQLiteCfg(p string) *SQLiteConfig {
	return &SQLiteConfig{
		TableMain:    config.DB.MainTable,
		TableDeleted: config.DB.DeletedTable,
		MaxBytesSize: config.DB.MaxBytesSize,
		Path:         p,
		Backup:       *newSQLiteBackup(p),
	}
}
