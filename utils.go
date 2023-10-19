package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

func shortenString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}
	return s
}

func toJSON(b *[]Bookmark) string {
	jsonData, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}
	jsonString := string(jsonData)
	return jsonString
}

func getAppHome() (string, error) {
	if ConfigHome == "" {
		ConfigHome = os.Getenv("HOME")
		ConfigHome += "/.config"
	}
	return filepath.Join(ConfigHome, AppName), nil
}

func getDBPath() (string, error) {
	appPath, err := getAppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(appPath, DBName), nil
}

func setupHomeProject() {
	AppHome, err := getAppHome()
	if err != nil {
		log.Fatal(err)
	}

	if !fileExists(AppHome) {
		log.Println("Creating AppHome:", AppHome)
		err = os.Mkdir(AppHome, 0755)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		return
	}
}

func isSelectedTextInItems(s string, items []string) bool {
	for _, item := range items {
		if strings.Contains(item, s) {
			return true
		}
	}
	return false
}

func findSelectedIndex(s string, items []string) int {
	for i, itemText := range items {
		if s == itemText {
			return i
		}
	}
	return -1
}

func prettyFormatLine(label, value string) string {
	return fmt.Sprintf("%-20s: %s\n", label, value)
}
