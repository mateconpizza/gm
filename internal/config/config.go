package config

import (
	"io"
	"log"
	"path/filepath"
)

var version = "0.1.11" // Version of the application

const (
	appName        string = "gomarks"      // Default name of the application
	command        string = "gm"           // Default name of the executable
	DefaultDBName  string = "bookmarks.db" // Default name of the database
	configFilename string = "config.yml"   // Default config filename
)

type (
	AppConfig struct {
		Name        string      `json:"name"`        // Name of the application
		Cmd         string      `json:"cmd"`         // Name of the executable
		Colorscheme string      `json:"colorscheme"` // Name of the colorscheme
		Version     string      `json:"version"`     // Version of the application
		Info        information `json:"data"`        // Application information
		Env         environment `json:"env"`         // Application environment variables
		Path        path        `json:"path"`        // Application path
		Color       bool        `json:"-"`           // Application color enable
		Force       bool        `json:"force"`       // force action, dont ask for confirmation.
		DBName      string      `json:"db"`          // Database name
		Verbose     bool        `json:"verbose"`     // Logging level
	}

	path struct {
		// Config string `json:"home"` // Path to store configuration (unused)
		Data         string `json:"data"`         // Path to store database
		ConfigFile   string `json:"config"`       // Path to config file
		Colorschemes string `json:"colorschemes"` // Path to store colorschemes
	}

	information struct {
		URL   string `json:"url"`   // URL of the application
		Title string `json:"title"` // Title of the application
		Tags  string `json:"tags"`  // Tags of the application
		Desc  string `json:"desc"`  // Description of the application
	}

	environment struct {
		Home   string `json:"home"`   // Environment variable for the home directory
		Editor string `json:"editor"` // Environment variable for the preferred editor
	}

	colorscheme struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
)

// SetColorSchemePath sets the colorscheme path.
func SetColorSchemePath(p string) {
	App.Path.Colorschemes = p
}

// EnableColor enables color output.
func EnableColor(enabled bool) {
	App.Color = enabled
}

// SetForce sets the force flag, this will skip the confirmation prompt.
func SetForce(f bool) {
	App.Force = f
}

// SetDBName sets the database name.
func SetDBName(s string) {
	App.DBName = s
}

// SetDataPath sets the app data path.
func SetDataPath(p string) {
	App.Path.Data = p
	App.Path.ConfigFile = filepath.Join(p, configFilename)
}

// SetLoggingLevel sets the logging level based on the verbose flag.
func SetLoggingLevel(b bool) {
	App.Verbose = b
	if b {
		log.SetPrefix(appName + ": ")
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("verbose mode: on")

		return
	}

	silentLogger := log.New(io.Discard, "", 0)
	log.SetOutput(silentLogger.Writer())
}
