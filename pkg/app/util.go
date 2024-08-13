package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/util"
	"github.com/haaag/gm/pkg/util/files"
)

// LoadPath loads the path to the application's home directory.
//
// If environment variable GOMARKS_HOME is not set, it uses XDG_CONFIG_HOME.
func LoadPath(a *App) error {
	envConfigHome, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("loading paths: %w", err)
	}

	envHome := util.GetEnv(a.Env.Home, envConfigHome)
	a.Path = filepath.Join(envHome, a.Name)
	log.Printf("setting app home: '%s'", a.Path)

	return nil
}

// CreatePaths creates the required paths for the application.
func CreatePaths(a *App, bkHome string) error {
	paths := []string{a.Path, bkHome}
	for _, path := range paths {
		if err := files.Mkdir(path); err != nil {
			return fmt.Errorf("setting up paths: %w", err)
		}
	}

	return nil
}

// PrettyVersion formats version in a pretty way.
func PrettyVersion(morePretty bool) string {
	name := color.Blue(name).Bold().String()
	if morePretty {
		name = color.Blue(Banner).String()
	}

	return fmt.Sprintf("%s v%s %s/%s\n", name, Version, runtime.GOOS, runtime.GOARCH)
}
