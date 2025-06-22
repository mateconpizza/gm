package config

import (
	"log/slog"
	"os"
	"path/filepath"
)

// version of the application.
var version = "0.1.15"

const (
	appName         string = "gomarks"      // Default name of the application
	command         string = "gm"           // Default name of the executable
	DefaultDBName   string = "bookmarks.db" // Default name of the database
	DefaultFilename string = "config.yml"   // Default config filename
	configFilename  string = "config.yml"   // Default config filename
)

type (
	AppConfig struct {
		Name    string      `json:"name"`    // Name of the application
		Cmd     string      `json:"cmd"`     // Name of the executable
		DBName  string      `json:"db"`      // Database name
		DBPath  string      `json:"db_path"` // Database path
		Info    information `json:"data"`    // Application information
		Env     environment `json:"env"`     // Application environment variables
		Path    path        `json:"path"`    // Application path
		Flags   *Flags      `json:"-"`
		Verbose bool        `json:"-"` // Logging level
	}

	path struct {
		// Config string `json:"home"` // Path to store configuration (unused)
		Data       string `json:"data"`   // Path to store database
		ConfigFile string `json:"config"` // Path to config file
		Backup     string `json:"backup"` // Path to store backups
		Git        string `json:"git"`    // Path to store git
		// Colorschemes string `json:"colorschemes"` // Path to store colorschemes
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
