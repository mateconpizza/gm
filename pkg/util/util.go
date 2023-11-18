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
	"gomarks/pkg/config"
	"gomarks/pkg/errs"

	"github.com/atotto/clipboard"
	"golang.org/x/exp/slices"
)

func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

// Returns the path to the application's home directory.
func GetAppHome() string {
	// TODO: maybe use `github.com/adrg/xdg`
	envHome := os.Getenv(config.App.Env.Home)

	if envHome == "" {
		envHome = os.Getenv("HOME")
		config.Files.ConfigDir = fmt.Sprintf("%s/.config", envHome)
	} else {
		config.Files.ConfigDir = envHome
	}

	return filepath.Join(config.Files.ConfigDir, config.App.Name)
}

func GetDBPath() string {
	appPath := GetAppHome()
	s := filepath.Join(appPath, config.DB.Name)
	log.Print("GetDBPath: ", s)

	return s
}

// Checks and creates the application's home directory.
// Returns the path to the application's home directory and any error encountered during the process.
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

func GetInput(prompt string) string {
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
	q := color.ColorizeBold(question, color.White)
	c := color.Colorize("[y/N]", color.Gray)
	prompt := fmt.Sprintf("\n%s %s: ", q, c)

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

func NewGetInput(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("%w", err)
	}

	text := scanner.Text()

	if text == "" {
		return "", errs.ErrNoIDorQueryPrivided
	}

	return text, nil
}
