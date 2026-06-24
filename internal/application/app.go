// Package application manages application-wide settings, paths, and environment variables.
package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
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
	OutputFormat   string = "frame"          // Default output format
	EnvHome        string = "GOMARKS_HOME"   // Default Environment variable for app home
	EnvEditor      string = "GOMARKS_EDITOR" // Default Environment variable for app editor
)

type (
	App struct {
		Name   string       `json:"name"          yaml:"-"`             // Name of the application
		Cmd    string       `json:"cmd"           yaml:"-"`             // Name of the executable
		DBName string       `json:"db"            yaml:"db,omitempty"`  // Database name
		Format string       `json:"format"        yaml:"format"`        // Output bookmark format
		Info   *Information `json:"data"          yaml:"-"`             // Application information
		Env    *Env         `json:"env"           yaml:"-"`             // Application environment variables
		Path   *Path        `json:"path"          yaml:"-"`             // Application path
		Flags  *Flags       `json:"-"             yaml:"-"`             // Command line flags
		Menu   *menu.Config `json:"menu"          yaml:"menu"`          // Menu configuration
		Git    *Git         `json:"git,omitempty" yaml:"git,omitempty"` // Git configuration

		initialized bool
	}

	Information struct {
		URL     string `json:"url"`     // Project homepage or repository URL.
		Title   string `json:"title"`   // Human-readable application title.
		Tags    string `json:"tags"`    // Comma-separated keywords describing the application.
		Desc    string `json:"desc"`    // Short application description.
		Version string `json:"version"` // Application semantic version.
		Commit  string `json:"commit"`  // Git commit hash used for the build.
		Date    string `json:"date"`    // Build timestamp in UTC.
	}

	Env struct {
		Home   string `json:"home"`   // Environment variable for the home directory
		Editor string `json:"editor"` // Environment variable for the preferred editor
	}
)

// Initialize prepares the config after flags are parsed.
func (app *App) Initialize() {
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

	// set main database path and name
	return app.SetDatabase(app.DBName)
}

// Load loads the user configurations file.
func (app *App) Load() error {
	err := getConfig(app.Path.ConfigFile(), app)
	if err != nil && !errors.Is(err, files.ErrFileNotFound) {
		return err
	}

	app.Git.Load()
	app.Flags.Output = app.Format

	return app.SetDatabase(app.DBName)
}

func (app *App) WriteConfig(force bool) error {
	app.DBName = app.DBBaseName()
	if !git.Initialized(app.Path.Git()) {
		app.Git = nil
	}

	return WriteYAML(app.Path.ConfigFile(), app, force)
}

// Validate validates the configuration file.
func (app *App) Validate() error {
	if app.DBName == "" {
		return ErrDatabaseNameNotSet
	}
	if files.StripSuffixes(app.DBName) == "" {
		return ErrDatabaseInvalidName
	}
	if app.Path.DB() == "" {
		return ErrDatabasePathNotSet
	}
	if app.Menu != nil {
		return app.Menu.Validate()
	}
	return nil
}

// PrettyVersion formats version information.
func (app *App) PrettyVersion() string {
	name := ansi.BrightBlue.Wrap(app.Name, ansi.Bold)

	ver := app.Version()
	if ver != "dev" {
		ver = "v" + ver
	}

	if app.Flags.Verbose == 0 {
		return fmt.Sprintf(
			"%s %s %s/%s\n",
			name,
			ver,
			runtime.GOOS,
			runtime.GOARCH,
		)
	}

	var sb strings.Builder

	fmt.Fprintf(&sb, "%s %s\n\n", name, ver)

	w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)

	if app.Info.Commit != "" && app.Info.Commit != "none" {
		fmt.Fprintf(w, "commit:\t%s\n", app.Info.Commit)
	}

	if app.Info.Date != "" && app.Info.Date != "unknown" {
		fmt.Fprintf(w, "built:\t%s\n", app.Info.Date)
	}

	fmt.Fprintf(w, "go version:\t%s\n", runtime.Version())
	fmt.Fprintf(w, "platform:\t%s/%s\n", runtime.GOOS, runtime.GOARCH)

	_ = w.Flush()

	return sb.String()
}

// SetDatabase sets the database name and path.
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

func (app *App) DBBaseName() string { return files.StripSuffixes(app.DBName) }
func (app *App) CreatePaths() error { return app.Path.setup() }
func (app *App) GitEnabled() bool   { return app.Git.Enabled }
func (app *App) Version() string    { return app.Info.Version }
func (app *App) Command() string    { return app.Cmd }

func (app *App) Example(template string) string {
	return strings.NewReplacer(
		"{cmd}", app.Cmd,
		"{version}", app.Version(),
		"{db}", app.DBBaseName(),
	).Replace(template)
}

func New(info *Information) *App {
	return &App{
		Name:   Name,
		Cmd:    Command,
		DBName: MainDBName,
		Format: OutputFormat,
		Flags:  &Flags{},
		Info:   info,
		Path:   &Path{},
		Git: &Git{
			Enabled: false,
			Log:     true,
			writer:  os.Stdout,
		},
		Env: &Env{
			Home:   EnvHome,
			Editor: EnvEditor,
		},
		Menu: menu.NewDefaultConfig(),
	}
}

func NewApp(dataHome string) *App {
	return &App{
		Path: &Path{
			Data: dataHome,
		},
	}
}

func NewInfo(version, commit, date string) *Information {
	return &Information{
		URL:     "https://github.com/mateconpizza/gm",
		Title:   "Gomarks: A bookmark manager",
		Tags:    "awesome,bookmarks,cli,golang",
		Desc:    "Simple yet powerful bookmark manager for your terminal",
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}
