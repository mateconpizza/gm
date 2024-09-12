package menu

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/haaag/gm/internal/config"
)

var Command = config.App.Cmd

func appendKeyDescToHeader(opts []string, key, desc string) []string {
	return append(opts, fmt.Sprintf("%s: %s", key, desc))
}

func formatterToStr[T any](s T) string {
	return fmt.Sprint(s)
}

func removeANSICodes(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func exitWithErrCode(code int, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("fzf: %w", err))
	}
	os.Exit(code)
}

func formatItems[T any](items []T, preprocessor func(T) string) chan string {
	inputChan := make(chan string)
	go func() {
		for _, item := range items {
			formatted := preprocessor(item)
			inputChan <- formatted
		}
		close(inputChan)
	}()

	return inputChan
}

func processOutput[T any](
	items []T,
	preprocessor func(T) string,
	outputChan <-chan string,
	resultChan chan<- []T,
) {
	var result []T
	ogItem := make(map[string]T)

	for _, item := range items {
		formatted := removeANSICodes(preprocessor(item))
		ogItem[formatted] = item
	}

	for s := range outputChan {
		if item, exists := ogItem[s]; exists {
			result = append(result, item)
		}
	}
	resultChan <- result
}

func loadHeader(header []string, args *[]string) {
	if len(header) == 0 {
		return
	}

	h := strings.Join(header, " ╱ ")
	*args = append(*args, "--header="+h)
}

func loadKeybind(keybind []string, args *[]string) {
	if len(keybind) == 0 {
		return
	}

	keys := strings.Join(keybind, ",")
	*args = append(*args, "--bind", keys)
}

// withCommand formats string with the name of the Command, the same name
// used when building the binary.
func withCommand(s string) string {
	return fmt.Sprintf(s, Command)
}
