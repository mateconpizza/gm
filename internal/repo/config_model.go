// Package repo provides the model of the configuration for a database.
package repo

import (
	"path/filepath"

	"github.com/haaag/gm/internal/util/files"
)

// SQLiteConfig represents the configuration for a SQLite database.
type SQLiteConfig struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Type         string `json:"type"`
	TableMain    string `json:"table_main"`
	TableDeleted string `json:"table_deleted"`
	BackupPath   string `json:"backup_path"`
	MaxBytesSize int64  `json:"max_bytes_size"`
	MaxBackups   int    `json:"max_backups_allowed"`
}

func (c *SQLiteConfig) Fullpath() string {
	return filepath.Join(c.Path, c.Name)
}

func (c *SQLiteConfig) GetTableMain() string {
	return c.TableMain
}

func (c *SQLiteConfig) GetTableDeleted() string {
	return c.TableDeleted
}

func (c *SQLiteConfig) SetPath(path string) *SQLiteConfig {
	c.Path = path
	return c
}

func (c *SQLiteConfig) SetName(name string) *SQLiteConfig {
	c.Name = files.EnsureExtension(name, ".db")
	return c
}

// NewSQLiteCfg returns the default settings for the database.
func NewSQLiteCfg(p string) *SQLiteConfig {
	// FIX: too complicated to setup a SQLiteConfig.
	return &SQLiteConfig{
		TableMain:    DatabaseMainTable,
		TableDeleted: DatabaseDeletedTable,
		Type:         "sqlite",
		MaxBytesSize: MaxBytesSize,
		Path:         p,
		BackupPath:   filepath.Join(p, "backup"),
	}
}
