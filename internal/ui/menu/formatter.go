package menu

import (
	"fmt"
	"strings"

	shellwords "github.com/junegunn/go-shellwords"
)

// appendKeytoHeader appends a key:desc string to the header slice.
func appendKeytoHeader(opts []string, key, desc string) []string {
	if !menuConfig.Header.Enabled {
		return opts
	}

	return append(opts, fmt.Sprintf("%s:%s", key, desc))
}

// defaultPreprocessor provides fallback formatting item for display in fzf.
func defaultPreprocessor[T any](item *T) string {
	return fmt.Sprintf("%+v", *item)
}

// formatItemsPreprocessed formats each item in the slice using the preprocessor function
// and returns a channel of formatted strings.
func formatItemsPreprocessed(formattedItems []string) chan string {
	inputChan := make(chan string)

	go func() {
		for _, formatted := range formattedItems {
			inputChan <- formatted
		}
		close(inputChan)
	}()

	return inputChan
}

// processOutputPreprocessed formats items, maps them to their original values, and sends
// the filtered results to resultChan.
func processOutputPreprocessed[T any](itemMap map[string]T, outputChan <-chan string, resultChan chan<- []T) {
	var result []T

	for s := range outputChan {
		if item, exists := itemMap[s]; exists {
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
