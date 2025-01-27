package menu

import (
	"errors"
	"fmt"
	"strings"

	shellwords "github.com/junegunn/go-shellwords"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
)

var (
	ErrFzfNoMatching          = errors.New("fzf: no matching record: code 1")
	ErrFzf                    = errors.New("fzf: error: code 2")
	ErrFzfPermissionDenied    = errors.New("fzf: permission denied from become action: code 127")
	ErrFzfInvalidShellCommand = errors.New("fzf: invalid shell command for become action: code 126")
)

// appendKeytoHeader appends a key:desc string to the header slice.
func appendKeytoHeader(opts []string, key, desc string) []string {
	if !menuConfig.Header {
		return opts
	}

	return append(opts, fmt.Sprintf("%s:%s", key, desc))
}

// toString converts any item to a string.
func toString[T any](s T) string {
	return fmt.Sprintf("%+v", s)
}

// formatItems formats each item in the slice using the preprocessor function
// and returns a channel of formatted strings.
func formatItems[T any](items []T, preprocessor func(*T) string) chan string {
	inputChan := make(chan string)
	go func() {
		for _, item := range items {
			formatted := preprocessor(&item)
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
	var result []T
	ogItem := make(map[string]T)

	for _, item := range items {
		formatted := color.RemoveANSICodes(preprocessor(&item))
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

// handleFzfErr returns an error based on the exit code of fzf.
func handleFzfErr(retcode int) error {
	/*
	 * 0      Normal exit
	 * 1      No match
	 * 2      Error
	 * 126    Permission denied error from become action
	 * 127    Invalid shell command for become action
	 * 130    Interrupted with CTRL-C or ESC
	 */
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
