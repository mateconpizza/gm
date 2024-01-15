package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return def
}

func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

// loadAppPaths loads the path to the application's home directory.
func loadAppPaths() {
	envConfigHome, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	envHome := getEnv(App.Env.Home, envConfigHome)
	App.Path.Home = filepath.Join(envHome, App.Name)
	App.Path.Backup = filepath.Join(App.Path.Home, "backup")
	log.Println("appHome:", App.Path.Home)
}

// LoadRepoPath loads the path to the database
func LoadRepoPath() {
	loadAppPaths()

	DB.Path = filepath.Join(App.Path.Home, DB.Name)
	log.Print("db path: ", DB.Path)
}

// SetupProjectPaths checks and creates the application's home directory.
func SetupProjectPaths() error {
	const dirPermissions = 0o755

	loadAppPaths()

	h := App.Path.Home

	if !fileExists(h) {
		log.Println("creating apphome:", h)
		err := os.Mkdir(h, dirPermissions)
		if err != nil {
			return fmt.Errorf("error creating apphome: %w", err)
		}
	}

	log.Println("appHome already exists:", h)

	return nil
}
