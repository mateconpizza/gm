// Package repo provides the model of the configuration for a database.
package repo

import (
	"path/filepath"

	"github.com/haaag/gm/internal/sys/files"
)

const maxBytesSize = 1024 * 1024 // 1 MB = 1,048,576 bytes

// SQLiteConfig represents the configuration for a SQLite database.
type SQLiteConfig struct {
	Name         string       `json:"name"`           // Name of the SQLite database
	Path         string       `json:"path"`           // Path to the SQLite database
	Tables       tables       `json:"tables"`         // Database tables
	Backup       SQLiteBackup `json:"backup"`         // Backup settings
	MaxBytesSize int64        `json:"max_bytes_size"` // Maximum size of the SQLite database
}

// tables represents the names of the tables in the SQLite database.
type tables struct {
	Main               Table `json:"main"`                 // Name of the main bookmarks table.
	Deleted            Table `json:"deleted"`              // Name of the deleted bookmarks table.
	Tags               Table `json:"tags"`                 // Name of the tags table
	RecordsTags        Table `json:"records_tags"`         // Name of the bookmark tags table
	RecordsTagsDeleted Table `json:"deleted_records_tags"` // Name of the deleted tags table
}

type SQLiteBackup struct {
	Path       string   `json:"path"`        // Path to store backups
	Files      []string `json:"files"`       // List of backup files
	Limit      int      `json:"limit"`       // Maximum number of backups
	DateFormat string   `json:"date_format"` // Date format
	Enabled    bool     `json:"enabled"`     // Backup enabled
}

// SetLimit sets the maximum number of backups.
func (b *SQLiteBackup) SetLimit(n int) {
	b.Limit = n
	b.Enabled = n > 0
}

// newSQLiteBackup returns a new SQLiteBackup.
func newSQLiteBackup(p string) *SQLiteBackup {
	return &SQLiteBackup{
		Path:       p,
		Files:      []string{},
		Enabled:    false,
		DateFormat: "2006-01-02_15-04",
		Limit:      3,
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
func NewSQLiteCfg(p string) *SQLiteConfig {
	backupPath := filepath.Join(p, "backup")
	return &SQLiteConfig{
		Tables: tables{
			Main:               "bookmarks",
			Deleted:            "deleted_bookmarks",
			Tags:               "tags",
			RecordsTags:        "bookmark_tags",
			RecordsTagsDeleted: "deleted_records_tags",
		},
		Path:         p,
		Backup:       *newSQLiteBackup(backupPath),
		MaxBytesSize: maxBytesSize,
	}
}
