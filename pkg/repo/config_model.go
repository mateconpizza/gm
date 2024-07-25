package repo

import (
	"path/filepath"
	"strconv"

	"github.com/haaag/gm/pkg/util"
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

func (c *SQLiteConfig) SetPath(path string) {
	c.Path = path
}

func (c *SQLiteConfig) SetName(name string) {
	c.Name = util.EnsureDBSuffix(name)
}

// SetDefaults sets path/name to the repository
// and loads the max backup allowed (default: 3).
func (c *SQLiteConfig) SetDefaults(path, name, bkMaxEnv string) {
	c.SetPath(path)
	c.SetName(name)
	c.BackupPath = filepath.Join(path, "backup")
	c.setBackupMax(bkMaxEnv, DefBackupMax)
}

// setBackupMax loads the max backups allowed from a env var
// defaults to 3.
func (c *SQLiteConfig) setBackupMax(env string, fallback int) {
	defaultMax := strconv.Itoa(fallback)
	maxBackups, err := strconv.Atoi(util.GetEnv(env, defaultMax))
	if err != nil {
		c.MaxBackups = fallback
	}
	c.MaxBackups = maxBackups
}

// NewSQLiteCfg returns the default settings for the database.
func NewSQLiteCfg() *SQLiteConfig {
	return &SQLiteConfig{
		TableMain:    _defMainTable,
		TableDeleted: _defDeletedTable,
		Type:         "sqlite",
		MaxBytesSize: _defMaxBytesSize,
	}
}
