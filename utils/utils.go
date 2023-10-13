package utils

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gomarks/database"
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

func GetDatabasePath(dbName string) string {
	_, currentFile, _ := getCurrentFile()
	parentDir := filepath.Dir(currentFile)
	dbPath := filepath.Join(parentDir, dbName)
	log.Println("DBPATH", dbPath)
	return dbPath
}

func FolderExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func RegisterMenu(menuName string, command []string) {
	log.Printf("Registering menu: %s", menuName)
	Menus[menuName] = command
}

func GetMenu(menuName string) ([]string, error) {
	menu, ok := Menus[menuName]
	if !ok {
		return nil, fmt.Errorf("Menu '%s' not found", menuName)
	}
	return menu, nil
}

func PrettyPrintBookmark(bookmark *database.Bookmark) {
	fmt.Printf("ID: %-4d\nTitle: %s\nURL: %s\nTags: %s\nDesc: %s\n",
		bookmark.ID, validString(bookmark.Title), bookmark.URL, validString(bookmark.Tags),
		validString(bookmark.Desc))
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

	// Create a pipe to send the list of elements as input to the dmenu process
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal("Error creating pipe:", err)
	}

	// Create a pipe to capture the standard output of the dmenu process
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("Error creating output pipe:", err)
	}

	// Start the dmenu process
	err = cmd.Start()
	if err != nil {
		log.Fatal("Error starting dmenu:", err)
	}

	// Create a string that contains text representations of the elements
	var itemsText []string
	for _, bm := range *bookmarks {
		// Here, build a text representation of each element according to your needs
		itemText := fmt.Sprintf("%-4d %-80s %-10s", bm.ID, shortenString(bm.URL, 80), validString(bm.Tags))
		itemsText = append(itemsText, itemText)
	}

	// Convert the list of text representations into a string with line breaks
	itemsString := strings.Join(itemsText, "\n")

	// Send the list as input to the dmenu process
	_, err = stdinPipe.Write([]byte(itemsString))
	if err != nil {
		log.Fatal("Error writing to pipe:", err)
	}

	// Close the standard input of the dmenu process
	stdinPipe.Close()

	// Capture the output of dmenu
	output, err := io.ReadAll(stdoutPipe)
	if err != nil {
		log.Fatal("Error reading dmenu output:", err)
	}

	// Wait for the dmenu process to finish
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
	if bookmarks == nil {
		return "[]"
	}
	actualBookmarks := *bookmarks
	jsonData, err := json.MarshalIndent(actualBookmarks, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}
	jsonString := string(jsonData)
	fmt.Println(jsonString)
	return jsonString
}
