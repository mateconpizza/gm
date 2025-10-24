package menu

import (
	"fmt"
)

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
