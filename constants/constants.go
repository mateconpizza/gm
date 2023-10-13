package constants

import (
	"fmt"
	"gomarks/utils"
	"log"
	"os"
	"path/filepath"
)

var dbPath string
var dbName string = "bookmarks.db"
var configHome string = os.Getenv("XDG_CONFIG_HOME")
var appName = "GoBookmarks"

func getAppHome() (string, error) {
	if configHome == "" {
		return "", fmt.Errorf("XDG_CONFIG_HOME not set")
	}
	return filepath.Join(configHome, appName), nil
}

func GetDatabasePath() (string, error) {
	appPath, err := getAppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(appPath, dbName), nil
}

func SetupProject() {
	AppHome, err := getAppHome()
	if err != nil {
		log.Fatal(err)
	}

	if !utils.FolderExists(AppHome) {
		log.Println("Creating AppHome:", AppHome)
		err = os.Mkdir(AppHome, 0755)
		if err != nil {
			log.Fatal(err)
		}
	} else {
    return
	}
}
