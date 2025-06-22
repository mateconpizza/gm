package menu

import (
	"fmt"
	"strings"

	shellwords "github.com/junegunn/go-shellwords"

	"github.com/mateconpizza/gm/internal/ui/color"
)

// appendKeytoHeader appends a key:desc string to the header slice.
func appendKeytoHeader(opts []string, key, desc string) []string {
	if !menuConfig.Header.Enabled {
		return opts
	}

	return append(opts, fmt.Sprintf("%s:%s", key, desc))
}

func defaultPreprocessor[T any](item *T) string {
	return fmt.Sprintf("%+v", *item)
}

// formatItems formats each item in the slice using the preprocessor function
// and returns a channel of formatted strings.
func formatItems[T any](items []T, preprocessor func(*T) string) chan string {
	inputChan := make(chan string)
	go func() {
		for _, item := range items {
			ti := item
			formatted := preprocessor(&ti)
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
	preprocessor func(*T) string,
	outputChan <-chan string,
	resultChan chan<- []T,
) {
	var (
		result []T
		ogItem = make(map[string]T)
	)

	for _, item := range items {
		ti := item
		formatted := color.ANSICodeRemover(preprocessor(&ti))
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
func loadHeader(header []string, args *FzfSettings) {
	if len(header) == 0 {
		return
	}

	h := strings.Join(header, menuConfig.Header.Sep)
	*args = append(*args, "--header="+h)
}

// loadKeybind appends a comma-separated keybind string to args.
func loadKeybind(keybind []string, args *FzfSettings) error {
	if len(keybind) == 0 {
		return nil
	}

	keys := strings.Join(keybind, ",")

	a, err := shellwords.Parse(fmt.Sprintf("%s=%q", "--bind", keys))
	if err != nil {
		return fmt.Errorf("parsing keybinds args: %w", err)
	}

	*args = append(*args, a...)

	return nil
}
