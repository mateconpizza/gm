package util

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gomarks/pkg/color"
	"gomarks/pkg/constants"

	"golang.org/x/exp/slices"
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

func GetAppHome() string {
	if constants.ConfigHome == "" {
		constants.ConfigHome = os.Getenv("HOME")
		constants.ConfigHome += "/.config"
	}
	s := filepath.Join(constants.ConfigHome, strings.ToLower(constants.AppName))
	return s
}

func GetDBPath() string {
	appPath := GetAppHome()
	s := filepath.Join(appPath, constants.DBName)
	log.Print("GetDBPath: ", s)
	return s
}

func SetupHomeProject() {
	AppHome := GetAppHome()

	if !FileExists(AppHome) {
		log.Println("Creating AppHome:", AppHome)
		err := os.Mkdir(AppHome, 0o755)
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Println("AppHome already exists:", AppHome)
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
	log.Printf("Finding selected in %d items", len(items))
	idx := slices.IndexFunc(items, func(item string) bool {
		return strings.Contains(item, s)
	})
	log.Println("FindSelectedIndex:", idx)
	return idx
}

func FormatTitleLine(n int, title, c string) string {
	if title == "" {
		title = "Untitled"
	}
	if c == "" {
		return fmt.Sprintf("%-4d\t%s %s\n", n, constants.BulletPoint, title)
	}
	return fmt.Sprintf(
		"%s%-4d\t%s%s %s%s\n",
		color.Bold,
		n,
		constants.BulletPoint,
		c,
		title,
		color.Reset,
	)
}

func FormatLine(prefix, v, c string) string {
	if c == "" {
		return fmt.Sprintf("%s%s\n", prefix, v)
	}
	return fmt.Sprintf("%s%s%s%s\n", c, prefix, v, color.Reset)
}

func SetLogLevel(verboseFlag *bool) {
	if *verboseFlag {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Verbose mode")
		return
	}
	silentLogger := log.New(io.Discard, "", 0)
	log.SetOutput(silentLogger.Writer())
}

func ReplaceArg(args []string, argName, newValue string) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == argName {
			args[i+1] = newValue
			break
		}
	}
}

func SplitAndAlignString(s string, lineLength int) string {
	words := strings.Fields(s)
	var result string
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 > lineLength {
			result += currentLine + "\n"
			currentLine = word
			currentLine = fmt.Sprintf("\t%s", currentLine)
		} else {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		}
	}

	result += currentLine
	return result
}

func binaryExists(binaryName string) bool {
	cmd := exec.Command("which", binaryName)
	err := cmd.Run()
	return err == nil
}

func ReadFile(file string) []byte {
	content, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	return content
}

func IsSameContentBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}

func EditFile(file string) error {
	editor, err := getEditor()
	if err != nil {
		return err
	}
	cmd := exec.Command(editor, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getEditor() (string, error) {
	GomarksEditor := os.Getenv("GOMARKS_EDITOR")
	if GomarksEditor != "" {
		log.Printf("Var $GOMARKS_EDITOR set to %s", GomarksEditor)
		return GomarksEditor, nil
	}

	Editor := os.Getenv("EDITOR")
	if Editor != "" {
		log.Printf("Var $EDITOR set to %s", Editor)
		return Editor, nil
	}

	log.Printf("Var $EDITOR not set.")
	if binaryExists("vim") {
		return "vim", nil
	}
	if binaryExists("nano") {
		return "nano", nil
	}
	if binaryExists("nvim") {
		return "nvim", nil
	}
	if binaryExists("emacs") {
		return "emacs", nil
	}
	return "", fmt.Errorf("no editor found")
}

func PrintErrMsg(m error, verbose bool) {
	if verbose {
		log.Fatal(m)
	}
	fmt.Printf("%s: %s\n", constants.AppName, m.Error())
	os.Exit(1)
}

func IsEmptyLine(line string) bool {
	return strings.TrimSpace(line) == ""
}
