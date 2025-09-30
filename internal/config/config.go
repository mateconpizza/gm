// Package config manages application-wide settings, paths, and environment variables.
package config

import (
	"path/filepath"
)

const (
	AppName        string = "gomarks"    // Default name of the application
	AppCommand     string = "gm"         // Default name of the executable
	MainDBName     string = "main.db"    // Default name of the main database
	ConfigFilename string = "config.yml" // Default config filename
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
		initialized bool
	}

	Path struct {
		Data       string `json:"data"`   // Path to store database
		ConfigFile string `json:"config"` // Path to config file
		Backup     string `json:"backup"` // Path to store backups
		Git        string `json:"git"`    // Path to store git
	}

	Git struct {
		Path    string `json:"path"`    // Path to store git
		Enabled bool   `json:"enabled"` // Enable git
		GPG     bool   `json:"gpg"`     // Enable GPG
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
