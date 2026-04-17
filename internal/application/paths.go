package application

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	gap "github.com/muesli/go-app-paths"
	yaml "gopkg.in/yaml.v3"

	"github.com/mateconpizza/gm/pkg/files"
)

func (app *App) CreatePaths() error {
	return files.MkdirAll(app.Path.Data)
}

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

// InitPaths initializes all filesystem paths for the application.
func (app *App) InitPaths() error {
	dataHomePath, err := loadDataPath(app.Name, app.Env.Home)
	if err != nil {
		return err
	}

	// set app home
	app.Path.Data = dataHomePath
	app.Path.ConfigFile = filepath.Join(app.Path.Data, ConfigFilename)
	app.Path.Backup = filepath.Join(app.Path.Data, "backup")

	// set main database path and name
	if filepath.Ext(app.DBName) != ".db" {
		app.DBName += ".db"
	}
	app.DBPath = filepath.Join(dataHomePath, app.DBName)

	return nil
}

// Load loads the user configurations file.
func Load(app *App) error {
	err := getConfig(app.Path.ConfigFile, app)
	if err != nil && !errors.Is(err, files.ErrFileNotFound) {
		return err
	}

	return nil
}

// getConfig loads the config file.
func getConfig(p string, app *App) error {
	if !files.Exists(p) {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	if err := ReadYAML(p, &app); err != nil {
		return fmt.Errorf("%w", err)
	}

	if app == nil {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	return app.Validate()
}

// ReadYAML unmarshals the YAML data from the specified file.
func ReadYAML[T any](p string, v *T) error {
	if !files.Exists(p) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, p)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	err = yaml.Unmarshal(content, &v)
	if err != nil {
		return fmt.Errorf("error unmarshalling YAML: %w", err)
	}

	slog.Debug("YamlRead", "path", p)

	return nil
}

// WriteYAML writes the provided YAML data to the specified file.
func WriteYAML[T any](p string, v *T, force bool) error {
	f, err := files.Touch(p, force)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("Yaml closing file", "file", p, "error", err)
		}
	}()

	data, err := yaml.Marshal(&v)
	if err != nil {
		return fmt.Errorf("error marshalling YAML: %w", err)
	}

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	slog.Info("YamlWrite success", "path", p)

	return nil
}
