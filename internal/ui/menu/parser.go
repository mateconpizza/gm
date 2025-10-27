package menu

import (
	"fmt"
	"log/slog"
	"regexp"
)

const ExitSuccess = 0

// handleFzfErr returns an error based on the exit code of fzf.
//
//	0      Normal exit
//	1      No match
//	2      Error
//	126    Permission denied error from become action
//	127    Invalid shell command for become action.
//	130    Interrupted with CTRL-C or ESC.
func handleFzfErr(retcode int) error {
	switch retcode {
	case 1:
		return ErrFzfNoMatching
	case 2:
		return ErrFzf
	case 126:
		return ErrFzfInvalidShellCommand
	case 127:
		return ErrFzfPermissionDenied
	case 130:
		return ErrFzfActionAborted
	}

	return nil
}

// selectFromItems runs Fzf with the given items and returns the selected item/s.
func selectFromItems[T comparable](m *Menu[T]) ([]T, error) {
	if len(m.items) == 0 {
		return nil, ErrFzfNoItems
	}

	if m.preprocessor == nil {
		slog.Warn("preprocessor is nil, defaulting to 'defaultPreprocessor'")

		m.preprocessor = defaultPreprocessor
	}

	slog.Debug("menu args", "args", m.args.list)

	// Pre-process all items once for better performance
	formattedItems := make([]string, len(m.items))
	itemMap := make(map[string]T, len(m.items))
	for i, item := range m.items {
		ti := item
		formatted := m.preprocessor(&ti)
		formattedItems[i] = formatted
		itemMap[ansiCodeRemover(formatted)] = item
	}

	// channels
	inputChan := formatItemsPreprocessed(formattedItems)
	outputChan := make(chan string)
	resultChan := make(chan []T)
	go processOutputPreprocessed(itemMap, outputChan, resultChan)

	// Build Fzf.Options
	options, err := m.runner.Parse(m.cfg.Defaults, m.args.build())
	if err != nil {
		return nil, fmt.Errorf("fzf: %w", err)
	}

	// Set up input and output channels
	options.Input = inputChan
	options.Output = outputChan

	// Run Fzf
	retcode, err := m.runner.Run(options)
	if retcode != ExitSuccess {
		// regardless of what kind of error, always call `callInterruptFn`
		err = handleFzfErr(retcode)
		m.callInterruptFn(err)

		return nil, err
	}

	close(outputChan)

	result := <-resultChan

	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return result, nil
}

// ansiCodeRemover removes ANSI codes from a given string.
func ansiCodeRemover(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}
