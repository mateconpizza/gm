package config

import (
	"log/slog"
	"os"
	"path/filepath"
)

// version of the application.
var version = "0.1.14"

const (
	appName         string = "gomarks"      // Default name of the application
	command         string = "gm"           // Default name of the executable
	DefaultDBName   string = "bookmarks.db" // Default name of the database
	DefaultFilename string = "config.yml"   // Default config filename
	configFilename  string = "config.yml"   // Default config filename
)

type (
	AppConfig struct {
		Name        string      `json:"name"`        // Name of the application
		Cmd         string      `json:"cmd"`         // Name of the executable
		Colorscheme string      `json:"colorscheme"` // Name of the colorscheme
		DBName      string      `json:"db"`          // Database name
		DBPath      string      `json:"db_path"`     // Database path
		Info        information `json:"data"`        // Application information
		Env         environment `json:"env"`         // Application environment variables
		Path        path        `json:"path"`        // Application path
		Color       bool        `json:"-"`           // Application color enable
		Force       bool        `json:"-"`           // force action, dont ask for confirmation.
		Verbose     bool        `json:"-"`           // Logging level
	}

	path struct {
		// Config string `json:"home"` // Path to store configuration (unused)
		Data         string `json:"data"`         // Path to store database
		ConfigFile   string `json:"config"`       // Path to config file
		Backup       string `json:"backup"`       // Path to store backups
		Git          string `json:"git"`          // Path to store git
		Colorschemes string `json:"colorschemes"` // Path to store colorschemes
	}

	information struct {
		URL     string `json:"url"`     // URL of the application
		Title   string `json:"title"`   // Title of the application
		Tags    string `json:"tags"`    // Tags of the application
		Desc    string `json:"desc"`    // Description of the application
		Version string `json:"version"` // Version of the application
	}

	environment struct {
		Home   string `json:"home"`   // Environment variable for the home directory
		Editor string `json:"editor"` // Environment variable for the preferred editor
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

// SetDBPath sets the database fullpath.
func SetDBPath(p string) {
	App.DBPath = p
}

// SetPaths sets the app data path.
func SetPaths(p string) {
	App.Path.Data = p
	App.Path.ConfigFile = filepath.Join(p, configFilename)
	App.Path.Backup = filepath.Join(p, "backup")
	App.Path.Git = filepath.Join(p, "git")
}

func SetVerbosity(verbose int) {
	levels := []slog.Level{
		slog.LevelError,
		slog.LevelWarn,
		slog.LevelInfo,
		slog.LevelDebug,
	}
	level := levels[min(verbose, len(levels)-1)]

	logger := slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
			Level:     level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == "source" {
					if source, ok := a.Value.Any().(*slog.Source); ok {
						dir, file := filepath.Split(source.File)
						source.File = filepath.Join(filepath.Base(filepath.Clean(dir)), file)

						return slog.Attr{Key: "source", Value: slog.AnyValue(source)}
					}
				}

				return a
			},
		}),
	)
	slog.SetDefault(logger)

	slog.Debug("logging", "level", level)
}
