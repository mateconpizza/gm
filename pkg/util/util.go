package util

import (
	"fmt"
	"gomarks/pkg/color"
	"gomarks/pkg/constants"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

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

func getAppHome() (string, error) {
	if constants.ConfigHome == "" {
		constants.ConfigHome = os.Getenv("HOME")
		constants.ConfigHome += "/.config"
	}
	s := filepath.Join(constants.ConfigHome, constants.AppName)
	return s, nil
}

func GetDBPath() (string, error) {
	appPath, err := getAppHome()
	if err != nil {
		return "", err
	}
	s := filepath.Join(appPath, constants.DBName)
	log.Print("GetDBPath: ", s)
	return s, nil
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

func FormatTitleLine(n int, v, c string) string {
	if v == "" {
		v = "Untitled"
	}
	if c == "" {
		return fmt.Sprintf(" %-4d %s %s\n", n, constants.BulletPoint, v)
	}
	return fmt.Sprintf(" %-4d %s%s%s %s%s\n", n, color.Bold, constants.BulletPoint, c, v, color.Reset)
}

func FormatLine(prefix, v, c string) string {
	if c == "" {
		return fmt.Sprintf("%s%s\n", prefix, v)
	}
	return fmt.Sprintf("%s%s%s%s\n", c, prefix, v, color.Reset)
}

func SetLogLevel(verboseFlag bool) {
	if verboseFlag {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("VVerbose mode")
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
			currentLine = fmt.Sprintf("        %s", currentLine)
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
