/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package util

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"gomarks/pkg/color"
	"gomarks/pkg/constants"

	"github.com/atotto/clipboard"
	"golang.org/x/exp/slices"
)

func fileExists(s string) bool {
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

func SetupHomeProject() (string, error) {
	const directoryPermissions = 0o755

	appHome := GetAppHome()

	if !fileExists(appHome) {
		log.Println("Creating AppHome:", appHome)
		err := os.Mkdir(appHome, directoryPermissions)
		if err != nil {
			return "", fmt.Errorf("error creating AppHome: %w", err)
		}
	}

	log.Println("AppHome already exists:", appHome)
	return appHome, nil
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
	var result string

	words := strings.Fields(s)
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 > lineLength {
			result += currentLine + "\n"
			currentLine = word
			currentLine = fmt.Sprintf("\t  %s", currentLine)
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

func BinaryExists(binaryName string) bool {
	cmd := exec.Command("which", binaryName)
	err := cmd.Run()

	return err == nil
}

func ReadContentFile(file *os.File) ([]byte, error) {
	tempFileName := file.Name()
	content, err := os.ReadFile(tempFileName)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return content, nil
}

func IsSameContentBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}

func IsEmptyLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

func ParseUniqueStrings(input []string, sep string) []string {
	uniqueTags := make([]string, 0)
	uniqueMap := make(map[string]struct{})

	for _, tags := range input {
		tagList := strings.Split(tags, sep)
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				uniqueMap[tag] = struct{}{}
			}
		}
	}

	for tag := range uniqueMap {
		uniqueTags = append(uniqueTags, tag)
	}

	return uniqueTags
}

func TakeInput(prompt string) string {
	var s string

	fmt.Printf("%s\n  > ", prompt)

	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	return strings.Trim(s, "\n")
}

func HandleInterrupt() <-chan struct{} {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})

	go func() {
		defer close(done)
		<-interrupt
		fmt.Println("\nReceived interrupt. Quitting...")
		os.Exit(1)
	}()

	return done
}

func Confirm(question string) bool {
	prompt := fmt.Sprintf(
		"\n%s%s %s[y/N]:%s ",
		question,
		color.Reset,
		color.Gray,
		color.Reset,
	)

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return false
		}

		input = strings.TrimSpace(input)
		input = strings.ToLower(input)

		switch input {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		case "":
			return false
		default:
			fmt.Println("Invalid response. Please enter 'Y' or 'n'.")
		}
	}
}

func CopyToClipboard(s string) {
	err := clipboard.WriteAll(s)
	if err != nil {
		log.Fatalf("Error copying to clipboard: %v", err)
	}

	log.Print("Text copied to clipboard:", s)
}
