package util

import (
	"fmt"
	"gomarks/pkg/color"
	c "gomarks/pkg/constants"
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
	if c.ConfigHome == "" {
		c.ConfigHome = os.Getenv("HOME")
		c.ConfigHome += "/.config"
	}
	s := filepath.Join(c.ConfigHome, c.AppName)
	return s, nil
}

func GetDBPath() (string, error) {
	appPath, err := getAppHome()
	if err != nil {
		return "", err
	}
	s := filepath.Join(appPath, c.DBName)
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
	} else {
		log.Println("AppHome already exists:", AppHome)
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
	log.Printf("Finding selected in %d items", len(items))
	idx := slices.IndexFunc(items, func(item string) bool {
		return strings.Contains(item, s)
	})
	log.Println("FindSelectedIndex:", idx)
	return idx
}

func PrettyFormatLine(label, value, c string) string {
	labelLength := 8
	if c == "" {
		return fmt.Sprintf(" %-*s: %s\n", labelLength, label, value)
	}
	return fmt.Sprintf(" %s%s%-*s:%s %s\n", color.Bold, c, labelLength, label, color.Reset, value)
}

func SetLogLevel(verboseFlag bool) {
	if verboseFlag {
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
