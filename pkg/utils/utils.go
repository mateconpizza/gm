package utils

import (
	"fmt"
	c "gomarks/pkg/constants"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func FileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

func ShortenString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}
	return s
}

func getAppHome() (string, error) {
	if c.ConfigHome == "" {
		c.ConfigHome = os.Getenv("HOME")
		c.ConfigHome += "/.config"
	}
	return filepath.Join(c.ConfigHome, c.AppName), nil
}

func GetDBPath() (string, error) {
	appPath, err := getAppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(appPath, c.DBName), nil
}

func SetupHomeProject() {
	AppHome, err := getAppHome()
	if err != nil {
		log.Fatal(err)
	}

	if !FileExists(AppHome) {
		log.Println("Creating AppHome:", AppHome)
		err = os.Mkdir(AppHome, 0755)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		return
	}
}

func IsSelectedTextInItems(s string, items []string) bool {
	for _, item := range items {
		if strings.Contains(item, s) {
			return true
		}
	}
	return false
}

func FindSelectedIndex(s string, items []string) int {
	for i, itemText := range items {
		if s == itemText {
			return i
		}
	}
	return -1
}

func PrettyFormatLine(label, value string) string {
	return fmt.Sprintf("%-20s: %s\n", label, value)
}
