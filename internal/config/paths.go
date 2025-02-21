package config

import (
	"fmt"
	"path/filepath"

	gap "github.com/muesli/go-app-paths"
)

// DataPath returns the data path for the application.
func DataPath() (string, error) {
	scope := gap.NewScope(gap.User, appName)
	dataDir, err := scope.DataPath("")
	if err != nil {
		return "", fmt.Errorf("getting data path: %w", err)
	}

	return dataDir, nil
}

// ConfigPath returns the config path for the application.
func ConfigPath() (string, error) {
	scope := gap.NewScope(gap.User, appName)
	configDir, err := scope.ConfigPath("")
	if err != nil {
		return "", fmt.Errorf("getting config path: %w", err)
	}

	return configDir, nil
}

// PathJoin returns the path joined with the application name.
func PathJoin(p string) string {
	return filepath.Join(p, appName)
}
