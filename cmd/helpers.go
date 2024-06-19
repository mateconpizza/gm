package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/editor"

	"github.com/atotto/clipboard"
)

var (
	ErrCopyToClipboard = errors.New("copy to clipboard")
)

// extractIDsFromStr extracts IDs from a string
func extractIDsFromStr(args []string) ([]int, error) {
	ids := make([]int, 0)

	// FIX: what is this!?
	for _, arg := range strings.Fields(strings.Join(args, " ")) {
		id, err := strconv.Atoi(arg)
		if err != nil {
			if errors.Is(err, strconv.ErrSyntax) {
				continue
			}
			return nil, fmt.Errorf("%w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// logErrAndExit logs the error and exits the program.
func logErrAndExit(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", App.Name, err)
	}
}

// setLoggingLevel sets the logging level based on the verbose flag.
func setLoggingLevel(verboseFlag *bool) {
	if *verboseFlag {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("verbose mode: on")

		return
	}

	silentLogger := log.New(io.Discard, "", 0)
	log.SetOutput(silentLogger.Writer())
}

// copyToClipboard copies a string to the clipboard
func copyToClipboard(s string) error {
	err := clipboard.WriteAll(s)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCopyToClipboard, err)
	}

	log.Print("text copied to clipboard:", s)
	return nil
}

// Open opens a URL in the default browser
func openBrowser(url string) error {
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"open"}
	case "windows":
		args = []string{"cmd", "/c", "start"}
	default:
		args = []string{"xdg-open"}
	}

	cmd := exec.Command(args[0], append(args[1:], url)...)
	err := cmd.Run()
	return fmt.Errorf("%w: opening in browser", err)
}

// filterBookmarkSelection select which item to remove from a slice using the
// text editor
func filterBookmarkSelection(bs *Slice) error {
	buf := bookmark.GetBufferSlice(bs)
	editor.AppendVersion(App.Name, App.Version, &buf)
	if err := editor.Edit(&buf); err != nil {
		return fmt.Errorf("on editing slice buffer: %w", err)
	}

	c := editor.Content(&buf)
	urls := editor.ExtractContentLine(&c)
	if len(urls) == 0 {
		return ErrActionAborted
	}

	bs.Filter(func(b Bookmark) bool {
		_, exists := urls[b.URL]
		return exists
	})

	return nil
}

// bookmarkEdition edits a bookmark with a text editor
func bookmarkEdition(b *Bookmark) error {
	buf := b.Buffer()
	if err := editor.Edit(&buf); err != nil {
		if errors.Is(err, editor.ErrBufferUnchanged) {
			return nil
		}
		return fmt.Errorf("%w", err)
	}
	content := editor.Content(&buf)
	tempB := bookmark.ParseContent(&content)
	if err := editor.Validate(&content); err != nil {
		return fmt.Errorf("%w", err)
	}

	tempB.ID = b.ID
	*b = *tempB

	return nil
}
