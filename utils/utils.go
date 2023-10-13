package utils

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gomarks/database"
  "gomarks/constants"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var Menus = make(map[string][]string)

func LoadMenus() {
	RegisterMenu("dmenu", []string{"dmenu", "-p", "GoMarks>", "-l", "10"})
	RegisterMenu("rofi", []string{
		"rofi", "-dmenu", "-p", "GoMarks>", "-l", "10", "-mesg",
		" > Welcome to GoMarks\n", "-theme-str", "window {width: 75%; height: 55%;}",
		"-kb-custom-1", "Alt-a"})
}

func getCurrentFile() (string, string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	exPath := filepath.Dir(ex)
	return ex, exPath, nil
}

func FolderExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func RegisterMenu(menuName string, command []string) {
	Menus[menuName] = command
}

func Menu(menuName string) ([]string, error) {
	menu, ok := Menus[menuName]
	if !ok {
		return nil, fmt.Errorf("Menu '%s' not found", menuName)
	}
	return menu, nil
}

func validString(title sql.NullString) string {
	if title.Valid {
		return title.String
	}
	return "N/A"
}

func shortenString(input string, maxLength int) string {
	if len(input) > maxLength {
		return input[:maxLength-3] + "..."
	}
	return input
}

func Prompt(menuArgs []string, bookmarks *[]database.Bookmark) (string, error) {
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
		itemText := fmt.Sprintf("%-4d %-80s %-10s", bm.ID, shortenString(bm.URL, 80), validString(bm.Tags))
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
		return "", fmt.Errorf("Program exited with non-zero status: %s", err)
	}

	// Extract the ID from the selected text (assuming the format is "ID - URL")
	selectedText := string(output)
	words := strings.Fields(selectedText)
	selectedID := words[0]
	return selectedID, nil
}

func ToJSON(bookmarks *[]database.Bookmark) string {
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
	if constants.ConfigHome == "" {
		return "", fmt.Errorf("XDG_CONFIG_HOME not set")
	}
	return filepath.Join(constants.ConfigHome, constants.AppName), nil
}

func GetDatabasePath() (string, error) {
	appPath, err := getAppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(appPath, constants.DBName), nil
}

func SetupProject() {
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
