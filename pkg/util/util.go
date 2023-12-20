// Copyright © 2023 haaag <git.haaag@gmail.com>
package util

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gomarks/pkg/config"
	"gomarks/pkg/format"

	"github.com/atotto/clipboard"
	"golang.org/x/exp/slices"
	"golang.org/x/term"
)

var ErrInvalidInput = errors.New("no id or query provided")

func CleanTerm() {
	fmt.Print("\033[H\033[2J")
	name := format.Text(config.App.Data.Title).Blue().Bold()
	fmt.Printf("%s: v%s\n\n", name, config.App.Version)
}

func FileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

func GetEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return def
}

// Loads the path to the application's home directory.
func LoadAppPaths() {
	// FIX: move to `config` pkg
	envConfigHome, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	envHome := GetEnv(config.App.Env.Home, envConfigHome)
	config.App.Path.Home = filepath.Join(envHome, config.App.Name)
	config.App.Path.Backup = filepath.Join(config.App.Path.Home, "backup")
	log.Println("AppHome:", config.App.Path.Home)
}

// Checks and creates the application's home directory.
// Returns the path to the application's home directory and any error encountered during the process.
func SetupProjectPaths() error {
	// FIX: move to `config` pkg
	const dirPermissions = 0o755

	LoadAppPaths()

	h := config.App.Path.Home

	if !FileExists(h) {
		log.Println("Creating AppHome:", h)
		err := os.Mkdir(h, dirPermissions)
		if err != nil {
			return fmt.Errorf("error creating AppHome: %w", err)
		}
	}

	log.Println("AppHome already exists:", h)

	return nil
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

func GetInputFromArgs(args []string) (string, error) {
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
		return "", ErrInvalidInput
	}

	return text, nil
}

func GetInputFromPrompt(prompt string) string {
	var s string

	fmt.Printf("%s\n  > ", prompt)

	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	return strings.Trim(s, "\n")
}

func Confirm(question string) bool {
	// FIX: Remove `colors`
	q := question
	c := format.Text("[y/N]").Gray()
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

// GetConsoleSize returns the visible dimensions of the given terminal.
func GetConsoleSize() (width, height int, err error) {
	fileDescriptor := int(os.Stdout.Fd())

	if !term.IsTerminal(fileDescriptor) {
		return 0, 0, config.ErrNotTTY
	}

	width, height, err = term.GetSize(fileDescriptor)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", config.ErrGetTermSize, err)
	}

	return width, height, nil
}

func IsOutputRedirected() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		log.Println("Error getting stdout file info:", err)
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) == 0
}

func isInputFromPipe() bool {
	fileInfo, _ := os.Stdin.Stat()
	return fileInfo.Mode()&os.ModeCharDevice == 0
}

func getQueryFromPipe(r io.Reader) string {
	var result string
	scanner := bufio.NewScanner(bufio.NewReader(r))
	for scanner.Scan() {
		line := scanner.Text()
		result += line
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading from pipe:", err)
		return ""
	}
	return result
}

func ReadInputFromPipe(args *[]string) {
	if !isInputFromPipe() {
		return
	}

	s := getQueryFromPipe(os.Stdin)
	*args = append(*args, s)
}
