package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func folderExists(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}

func shortenString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}
	return s
}

func executeCommand(m *Menu, s string) (string, error) {
	cmd := exec.Command(m.Command, m.Arguments...)

	if s != "" {
		cmd.Stdin = strings.NewReader(s)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("Error creating output pipe:", err)
	}

	err = cmd.Start()
	if err != nil {
		log.Fatal("Error starting dmenu:", err)
	}

	output, err := io.ReadAll(stdoutPipe)
	if err != nil {
		log.Fatal("Error reading output:", err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("program exited with non-zero status: %s", err)
	}
	return string(output), nil
}

func toJSON(b *[]Bookmark) string {
	actualBookmarks := *b
	jsonData, err := json.MarshalIndent(actualBookmarks, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}
	jsonString := string(jsonData)
	fmt.Println(jsonString)
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

	if !folderExists(AppHome) {
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
