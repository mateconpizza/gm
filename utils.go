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

	"github.com/atotto/clipboard"
)

var Menus = make(map[string][]string)

func LoadMenus() {
	RegisterMenu("dmenu", []string{"dmenu", "-p", "GoMarks>", "-l", "10"})
	RegisterMenu("rofi", []string{
		"rofi", "-dmenu", "-p", "GoMarks>", "-l", "10", "-mesg",
		" > Welcome to GoMarks\n", "-theme-str", "window {width: 75%; height: 55%;}",
		"-kb-custom-1", "Alt-a"})
}

func FolderExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func RegisterMenu(s string, command []string) {
	Menus[s] = command
}

func Menu(s string) ([]string, error) {
	menu, ok := Menus[s]
	if !ok {
		return nil, fmt.Errorf("Menu '%s' not found", s)
	}
	return menu, nil
}

func shortenString(input string, maxLength int) string {
	if len(input) > maxLength {
		return input[:maxLength-3] + "..."
	}
	return input
}

func Prompt(menuArgs []string, bookmarks *[]Bookmark) (string, error) {
	cmd := exec.Command(menuArgs[0], menuArgs[1:]...)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal("Error creating pipe:", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("Error creating output pipe:", err)
	}

	err = cmd.Start()
	if err != nil {
		log.Fatal("Error starting dmenu:", err)
	}

	var itemsText []string
	for _, bm := range *bookmarks {
		itemText := fmt.Sprintf(
			"%-4d %-80s %-10s",
			bm.ID,
			shortenString(bm.URL, 80),
			bm.Tags,
		)
		itemsText = append(itemsText, itemText)
	}

	itemsString := strings.Join(itemsText, "\n")

	_, err = stdinPipe.Write([]byte(itemsString))
	if err != nil {
		log.Fatal("Error writing to pipe:", err)
	}
	stdinPipe.Close()

	output, err := io.ReadAll(stdoutPipe)
	if err != nil {
		log.Fatal("Error reading dmenu output:", err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("program exited with non-zero status: %s", err)
	}

	// Extract the ID from the selected text (assuming the format is "ID - URL")
	selectedText := string(output)
	words := strings.Fields(selectedText)
	selectedID := words[0]
	return selectedID, nil
}

func ToJSON(bookmarks *[]Bookmark) string {
	actualBookmarks := *bookmarks
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

func GetDatabasePath() (string, error) {
	appPath, err := getAppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(appPath, DBName), nil
}

func SetupHomeProject() {
	AppHome, err := getAppHome()
	if err != nil {
		log.Fatal(err)
	}

	if !FolderExists(AppHome) {
		log.Println("Creating AppHome:", AppHome)
		err = os.Mkdir(AppHome, 0755)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		return
	}
}

func CopyToClipboard(s string) {
	err := clipboard.WriteAll(s)
	if err != nil {
		log.Fatalf("Error copying to clipboard: %v", err)
	}
	log.Println("Text copied to clipboard:", s)
}
