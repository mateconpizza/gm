package application

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	files "github.com/mateconpizza/gofiles"
	gap "github.com/muesli/go-app-paths"

	"github.com/mateconpizza/gm/pkg/bookio"
)

type Path struct {
	Data     string `json:"data"`  // Path to store database
	Database string `json:"store"` // Database fullpath
}

func NewPath() *Path { return &Path{} }

func (p *Path) Home() string       { return p.Data }
func (p *Path) Backup() string     { return filepath.Join(p.Data, "backup") }
func (p *Path) DB() string         { return p.Database }
func (p *Path) ConfigFile() string { return filepath.Join(p.Data, ConfigFilename) }
func (p *Path) Git() string        { return filepath.Join(p.Data, "git") }
func (p *Path) GitReadme(repoName string) string {
	return filepath.Join(p.Git(), repoName, bookio.ReadmeFile)
}

func (p *Path) setup() error { return files.MkdirAll(p.Home()) }

// dataPath returns the data path for the application.
func dataPath(appName string) (string, error) {
	scope := gap.NewScope(gap.User, appName)

	dataDir, err := scope.DataPath("")
	if err != nil {
		return "", fmt.Errorf("getting data path: %w", err)
	}

	return dataDir, nil
}

// LoadPath loads the path to the application's home directory.
//
// If environment variable GOMARKS_HOME is not set, uses the data user
// directory.
func loadDataPath(appName, envVar string) (string, error) {
	envDataHome := os.Getenv(envVar)
	if envDataHome != "" {
		slog.Debug("reading home env", envVar, envDataHome)

		return filepath.Join(envDataHome, appName), nil
	}

	dataHome, err := dataPath(appName)
	if err != nil {
		return "", fmt.Errorf("loading paths: %w", err)
	}

	slog.Debug("home app", "path", dataHome)

	return dataHome, nil
}

// getConfig loads the config file.
func getConfig(p string, app *App) error {
	if !files.Exists(p) {
		return fmt.Errorf("config %w", os.ErrNotExist)
	}

	if err := readYAML(p, &app); err != nil {
		return err
	}

	if app == nil {
		return fmt.Errorf("config %w", os.ErrNotExist)
	}

	return app.Validate()
}

// readYAML unmarshals the YAML data from the specified file.
func readYAML[T any](p string, v *T) error {
	if !files.Exists(p) {
		return fmt.Errorf("%w: %q", os.ErrNotExist, p)
	}

	f, err := os.Open(p)
	if err != nil {
		return err
	}

	return bookio.ReadYAML(f, &v)
}

// writeYAML writes the provided YAML data to the specified file.
func writeYAML[T any](p string, v *T, force bool) error {
	f, err := files.New(p, force)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}

	defer func() {
		if err = f.Close(); err != nil {
			slog.Error("Yaml closing file", "file", p, "error", err)
		}
	}()

	return bookio.WriteYAML(f, &v)
}
