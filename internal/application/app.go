// Package application manages application-wide settings, paths, and environment variables.
package application

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrDatabaseNameNotSet  = errors.New("database name not set")
	ErrDatabaseInvalidName = errors.New("database name invalid")
	ErrDatabasePathNotSet  = errors.New("database path not set")
)

const (
	Name           string = "gomarks"        // Default name of the application
	Command        string = "gm"             // Default name of the executable
	MainDBName     string = "main.db"        // Default name of the main database
	ConfigFilename string = "config.yml"     // Default config filename
	EnvHome        string = "GOMARKS_HOME"   // Default Environment variable for app home
	EnvEditor      string = "GOMARKS_EDITOR" // Default Environment variable for app editor
)

type (
	App struct {
		Name   string       `json:"name"          yaml:"-"`             // Name of the application
		Cmd    string       `json:"cmd"           yaml:"-"`             // Name of the executable
		DBName string       `json:"db"            yaml:"db,omitempty"`  // Database name
		Info   *Information `json:"data"          yaml:"-"`             // Application information
		Env    *Env         `json:"env"           yaml:"-"`             // Application environment variables
		Path   *Path        `json:"path"          yaml:"-"`             // Application path
		Flags  *Flags       `json:"-"             yaml:"-"`             // Command line flags
		Menu   *menu.Config `json:"menu"          yaml:"menu"`          // Menu configuration
		Git    *Git         `json:"git,omitempty" yaml:"git,omitempty"` // Git configuration

		initialized bool
	}

	Path struct {
		Data     string `json:"data"`   // Path to store database
		Config   string `json:"config"` // Path to config file
		Backup   string `json:"backup"` // Path to store backups
		Database string `json:"store"`  // Database path
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
func (app *App) Initialize() {
	// FIX: drop this, use Setup instead.
	if app.initialized {
		return
	}

	if err := app.SetDatabase(app.DBName); err != nil {
		panic(err)
	}

	app.initialized = true
}

// Setup initializes all filesystem paths for the application.
func (app *App) Setup() error {
	dataHomePath, err := loadDataPath(app.Name, app.Env.Home)
	if err != nil {
		return err
	}

	// set app home
	app.Path.Data = dataHomePath
	app.Path.Config = filepath.Join(app.Path.Data, ConfigFilename)
	app.Path.Backup = filepath.Join(app.Path.Data, "backup")

	// set main database path and name
	return app.SetDatabase(app.DBName)
}

// Load loads the user configurations file.
func (app *App) Load() error {
	err := getConfig(app.Path.Config, app)
	if err != nil && !errors.Is(err, files.ErrFileNotFound) {
		return err
	}

	return app.SetDatabase(app.DBName)
}

func (app *App) WriteConfig() error {
	app.DBName = files.StripSuffixes(app.DBName)
	if !app.Git.Enabled {
		app.Git = nil
	}

	return WriteYAML(app.Path.Config, app, app.Flags.Force)
}

// Validate validates the configuration file.
func (app *App) Validate() error {
	if app.DBName == "" {
		return ErrDatabaseNameNotSet
	}
	if files.StripSuffixes(app.DBName) == "" {
		return ErrDatabaseInvalidName
	}
	if app.Path.Database == "" {
		return ErrDatabasePathNotSet
	}
	if app.Menu != nil {
		return app.Menu.Validate()
	}
	return nil
}

func (app *App) PreviewCmd(dbPath string, args ...string) string {
	return fmt.Sprintf("%s --preview frame --db %s %s", app.Cmd, dbPath, strings.Join(args, " "))
}

// PrettyVersion formats version in a pretty way.
func (app *App) PrettyVersion() string {
	return fmt.Sprintf(
		"%s v%s %s/%s",
		ansi.BrightBlue.Wrap(app.Name, ansi.Bold),
		app.Info.Version,
		runtime.GOOS,
		runtime.GOARCH,
	)
}

func (app *App) SetDatabase(name string) error {
	app.DBName = files.StripSuffixes(name)
	if app.DBName == "" {
		return ErrDatabaseNameNotSet
	}
	if filepath.Ext(app.DBName) != ".db" {
		app.DBName += ".db"
	}
	if app.Path.Data == "" {
		return ErrDatabasePathNotSet
	}
	app.Path.Database = filepath.Join(app.Path.Data, app.DBName)
	return nil
}

func New(info *Information) *App {
	return &App{
		Name:   Name,
		Cmd:    Command,
		DBName: MainDBName,
		Flags:  &Flags{},
		Info:   info,
		Path:   &Path{},
		Git: &Git{
			Enabled: false,
			GPG:     false,
			// FIX: `Log` not implemented yet
			// if set to `false` it will silent the `git` output
			Log: true,
		},
		Env: &Env{
			Home:   EnvHome,
			Editor: EnvEditor,
		},
		Menu: menu.NewDefaultConfig(),
	}
}
