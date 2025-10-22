// Package config manages application-wide settings, paths, and environment variables.
package config

import (
	"path/filepath"

	"github.com/mateconpizza/gm/internal/ui/menu"
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
		Name        string       `json:"name"`    // Name of the application
		Cmd         string       `json:"cmd"`     // Name of the executable
		DBName      string       `json:"db"`      // Database name
		DBPath      string       `json:"db_path"` // Database path
		Info        *Information `json:"data"`    // Application information
		Env         *Env         `json:"env"`     // Application environment variables
		Path        *Path        `json:"path"`    // Application path
		Flags       *Flags       `json:"-"`       // Command line flags
		Verbose     bool         `json:"-"`       // Logging level
		Git         *Git         `json:"-"`       // Git configuration
		Menu        *menu.Config `json:"-"`       // Menu configuration
		initialized bool
	}

	Path struct {
		Data       string `json:"data"`   // Path to store database
		ConfigFile string `json:"config"` // Path to config file
		Backup     string `json:"backup"` // Path to store backups
		Git        string `json:"git"`    // Path to store git
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
