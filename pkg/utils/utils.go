package utils

import (
	"fmt"
	c "gomarks/pkg/constants"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
)

type Counter map[string]int

func (c Counter) Add(tags string) {
	for _, tag := range strings.Split(tags, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			c[tag]++
		}
	}
}

func (c Counter) GetCount(item string) int {
	return c[item]
}

func (c Counter) Remove(item string) {
	delete(c, item)
}

func (c Counter) ToStringSlice() []string {
	var results []string
	for tag, count := range c {
		results = append(results, fmt.Sprintf("%s (%d)", tag, count))
	}
	sort.Strings(results)
	return results
}

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

func PrettyFormatLine(label, value string) string {
	return fmt.Sprintf("%-20s: %s\n", label, value)
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
