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

func folderExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func registerMenu(s string, command []string) {
	Menus[s] = command
}

func getMenu(s string) ([]string, error) {
	menu, ok := Menus[s]
	if !ok {
		return nil, fmt.Errorf("menu '%s' not found", s)
	}
	return menu, nil
}

func shortenString(input string, maxLength int) string {
	if len(input) > maxLength {
		return input[:maxLength-3] + "..."
	}
	return input
}

func NewexecuteCommand(menuArgs []string, input string) (string, int, error) {
	cmd := exec.Command(menuArgs[0], menuArgs[1:]...)

	if input != "" {
		cmd.Stdin = strings.NewReader(input)
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
		return "", cmd.ProcessState.ExitCode(), fmt.Errorf(
			"program exited with non-zero status: %s",
			err,
		)
	}
	return string(output), cmd.ProcessState.ExitCode(), nil
}

func executeCommand(menuArgs []string, input string) (string, error) {
	cmd := exec.Command(menuArgs[0], menuArgs[1:]...)

	if input != "" {
		cmd.Stdin = strings.NewReader(input)
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
		return "", fmt.Errorf("XDG_CONFIG_HOME not set")
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

func isSelectedTextInItems(selectedText string, itemsText []string) bool {
	for _, item := range itemsText {
		if strings.Contains(item, selectedText) {
			return true
		}
	}
	return false
}

func findSelectedIndex(selectedStr string, itemsText []string) int {
	for index, itemText := range itemsText {
		if strings.Contains(selectedStr, itemText) {
			return index
		}
	}
	return -1
}
