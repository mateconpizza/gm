// Package config manages application-wide settings, paths, and environment variables.
package config

import (
	"errors"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/ui/menu"
)

var (
	ErrDatabaseNameNotSet = errors.New("database name not set")
	ErrDatabasePathNotSet = errors.New("database path not set")
)

const (
	AppName        string = "gomarks"        // Default name of the application
	AppCommand     string = "gm"             // Default name of the executable
	MainDBName     string = "main.db"        // Default name of the main database
	ConfigFilename string = "config.yml"     // Default config filename
	EnvHome        string = "GOMARKS_HOME"   // Default Environment variable for app home
	EnvEditor      string = "GOMARKS_EDITOR" // Default Environment variable for app editor
)

type (
	Config struct {
		Name        string       `json:"name"          yaml:"-"`             // Name of the application
		Cmd         string       `json:"cmd"           yaml:"-"`             // Name of the executable
		DBName      string       `json:"db"            yaml:"-"`             // Database name
		DBPath      string       `json:"db_path"       yaml:"-"`             // Database path
		Info        *Information `json:"data"          yaml:"-"`             // Application information
		Env         *Env         `json:"env"           yaml:"-"`             // Application environment variables
		Path        *Path        `json:"path"          yaml:"-"`             // Application path
		Flags       *Flags       `json:"-"             yaml:"-"`             // Command line flags
		Verbose     bool         `json:"-"             yaml:"-"`             // Logging level
		Menu        *menu.Config `json:"menu"          yaml:"menu"`          // Menu configuration
		Git         *Git         `json:"git,omitempty" yaml:"git,omitempty"` // Git configuration
		initialized bool
	}

	Path struct {
		Data       string `json:"data"`   // Path to store database
		ConfigFile string `json:"config"` // Path to config file
		Backup     string `json:"backup"` // Path to store backups
	}

	Git struct {
		Enabled bool   `json:"enabled" yaml:"enabled"` // Enable git
		Log     bool   `json:"logging" yaml:"logging"` // Enable logging
		GPG     bool   `json:"gpg"     yaml:"gpg"`     // Enable GPG
		Path    string `json:"path"    yaml:"path"`    // Path to store git
		Remote  string `json:"remote"  yaml:"remote"`  // Remote repo
	}

	Information struct {
		URL     string `json:"url"`     // URL of the application
		Title   string `json:"title"`   // Title of the application
		Tags    string `json:"tags"`    // Tags of the application
		Desc    string `json:"desc"`    // Description of the application
		Version string `json:"version"` // Version of the application
	}

	Env struct {
		Home   string `json:"home"`   // Environment variable for the home directory
		Editor string `json:"editor"` // Environment variable for the preferred editor
	}
)

// Initialize prepares the config after flags are parsed.
func (c *Config) Initialize() {
	if c.initialized {
		return
	}

	if filepath.Ext(c.DBName) != ".db" {
		c.DBName += ".db"
	}
	c.DBPath = filepath.Join(c.Path.Data, c.DBName)
	c.initialized = true
}

// Validate validates the configuration file.
func (c *Config) Validate() error {
	if c.DBName == "" {
		return ErrDatabaseNameNotSet
	}
	if c.DBPath == "" {
		return ErrDatabasePathNotSet
	}

	if c.Menu != nil {
		return c.Menu.Validate()
	}

	return nil
}

func New() *Config {
	return &Config{}
}
