// Package repo provides the model of the configuration for a database.
package repo

import (
	"path/filepath"

	"github.com/haaag/gm/internal/sys/files"
)

type Table string

const maxBytesSize = 1024 * 1024 // 1 MB = 1,048,576 bytes

// SQLiteConfig represents the configuration for a SQLite database.
type SQLiteConfig struct {
	Name         string       `json:"name"`           // Name of the SQLite database
	Path         string       `json:"path"`           // Path to the SQLite database
	Table        tables       `json:"tables"`         // Database tables
	Backup       SQLiteBackup `json:"backup"`         // Backup settings
	MaxBytesSize int64        `json:"max_bytes_size"` // Maximum size of the SQLite database
}

// tables represents the names of the tables in the SQLite database.
type tables struct {
	Main        Table `json:"main"`         // Name of the main bookmarks table.
	Tags        Table `json:"tags"`         // Name of the tags table
	RecordsTags Table `json:"records_tags"` // Name of the bookmark tags table
}

type SQLiteBackup struct {
	Path       string   `json:"path"`        // Path to store backups
	Files      []string `json:"files"`       // List of backup files
	DateFormat string   `json:"date_format"` // Date format
}

// NewSQLiteBackup returns a new SQLiteBackup.
func NewSQLiteBackup(from string) *SQLiteBackup {
	return &SQLiteBackup{
		Path:       from,
		Files:      getBackups(from),
		DateFormat: "20060102-150405",
	}
}

// Fullpath returns the full path to the SQLite database.
func (c *SQLiteConfig) Fullpath() string {
	return filepath.Join(c.Path, c.Name)
}

// SetPath sets the path to the SQLite database.
func (c *SQLiteConfig) SetPath(p string) *SQLiteConfig {
	c.Path = p
	return c
}

// SetName sets the name of the SQLite database.
func (c *SQLiteConfig) SetName(s string) *SQLiteConfig {
	c.Name = files.EnsureExt(s, ".db")
	return c
}

// Exists returns true if the SQLite database exists.
func (c *SQLiteConfig) Exists() bool {
	return files.Exists(c.Fullpath())
}

// NewSQLiteCfg returns the default settings for the database.
func NewSQLiteCfg(fullpath string) *SQLiteConfig {
	return &SQLiteConfig{
		Table: tables{
			Main:        schemaMain.name,
			Tags:        schemaTags.name,
			RecordsTags: schemaRelation.name,
		},
		Path:         filepath.Dir(fullpath),
		Name:         files.EnsureExt(filepath.Base(fullpath), ".db"),
		Backup:       *NewSQLiteBackup(filepath.Join(filepath.Dir(fullpath), "backup")),
		MaxBytesSize: maxBytesSize,
	}
}
