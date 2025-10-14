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
	cfgFile, err := getConfig(cfg.Path.ConfigFile)
	if err != nil && !errors.Is(err, files.ErrFileNotFound) {
		return fmt.Errorf("%w", err)
	}

	if cfgFile == nil {
		slog.Debug("configfile is empty or not found. loading defaults")
		return nil
	}

	cfg.Menu = cfgFile.Menu

	Fzf = cfgFile.Menu

	return nil
}

// getConfig loads the config file.
func getConfig(p string) (*ConfigFile, error) {
	if !files.Exists(p) {
		return nil, fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	var cfg *ConfigFile
	if err := readYAML(p, &cfg); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if cfg == nil {
		return nil, fmt.Errorf("config %w", files.ErrFileNotFound)
	}

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return cfg, nil
}

// readYAML unmarshals the YAML data from the specified file.
func readYAML[T any](p string, v *T) error {
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
