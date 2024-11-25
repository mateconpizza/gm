package menu

import (
	"fmt"
	"os"
	"strings"

	"github.com/junegunn/go-shellwords"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
)

// appendToHeader appends a key:desc string to the header slice.
func appendToHeader(opts []string, key, desc string) []string {
	return append(opts, fmt.Sprintf("%s:%s", key, desc))
}

// toString converts any item to a string.
func toString[T any](s T) string {
	return fmt.Sprint(s)
}

// formatItems formats each item in the slice using the preprocessor function
// and returns a channel of formatted strings.
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

// processOutput formats items, maps them to their original values, and sends
// the filtered results to resultChan.
func processOutput[T any](
	items []T,
	preprocessor func(T) string,
	outputChan <-chan string,
	resultChan chan<- []T,
) {
	var result []T
	ogItem := make(map[string]T)

	for _, item := range items {
		formatted := color.RemoveANSICodes(preprocessor(item))
		ogItem[formatted] = item
	}

	for s := range outputChan {
		if item, exists := ogItem[s]; exists {
			result = append(result, item)
		}
	}
	resultChan <- result
}

// loadHeader appends a formatted header string to args.
func loadHeader(header []string, args *[]string) {
	if len(header) == 0 {
		return
	}

	h := strings.Join(header, " "+format.BulletPoint+" ")
	*args = append(*args, "--header="+h)
}

// loadKeybind appends a comma-separated keybind string to args.
func loadKeybind(keybind []string, args *[]string) error {
	if len(keybind) == 0 {
		return nil
	}

	keys := strings.Join(keybind, ",")
	a, err := shellwords.Parse(fmt.Sprintf("%s='%s'", "--bind", keys))
	if err != nil {
		return fmt.Errorf("parsing keybinds args: %w", err)
	}
	*args = append(*args, a...)

	return nil
}

// withCommand formats string with the name of the Command, the same name
// used when building the binary.
func withCommand(s string) string {
	return fmt.Sprintf(s, config.App.Cmd)
}

// exitWithErrCode exits with an error code.
func exitWithErrCode(code int, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("fzf: %w", err))
	}
	os.Exit(code)
}
