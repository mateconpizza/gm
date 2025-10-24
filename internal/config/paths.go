package config

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

func (c *Config) CreatePaths() error {
	return files.MkdirAll(c.Path.Data)
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
func (c *Config) InitPaths() {
	dataHomePath, err := loadDataPath(c.Name, c.Env.Home)
	if err != nil {
		panic(err)
	}

	// set app home
	c.Path.Data = dataHomePath
	c.Path.ConfigFile = filepath.Join(c.Path.Data, ConfigFilename)
	c.Path.Backup = filepath.Join(c.Path.Data, "backup")

	// set main database path and name
	if filepath.Ext(c.DBName) != ".db" {
		c.DBName += ".db"
	}
	c.DBPath = filepath.Join(dataHomePath, c.DBName)
}

// Load loads the user configurations file.
func Load(cfg *Config) error {
	err := getConfig(cfg.Path.ConfigFile, cfg)
	if err != nil && !errors.Is(err, files.ErrFileNotFound) {
		return err
	}

	return nil
}

// getConfig loads the config file.
func getConfig(p string, cfg *Config) error {
	if !files.Exists(p) {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	if err := ReadYAML(p, &cfg); err != nil {
		return fmt.Errorf("%w", err)
	}

	if cfg == nil {
		return fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	return cfg.validate()
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
