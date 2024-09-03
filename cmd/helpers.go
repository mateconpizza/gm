package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/editor"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/qr"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util"
	"github.com/haaag/gm/pkg/util/spinner"
)

var ErrCopyToClipboard = errors.New("copy to clipboard")

type colorFn func(...string) *color.Color

// extractIDsFromStr extracts IDs from a string.
func extractIDsFromStr(args []string) ([]int, error) {
	ids := make([]int, 0)
	if len(args) == 0 {
		return ids, nil
	}

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
		os.Exit(1)
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

// copyToClipboard copies a string to the clipboard.
func copyToClipboard(s string) error {
	err := clipboard.WriteAll(s)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCopyToClipboard, err)
	}

	log.Print("text copied to clipboard:", s)

	return nil
}

// Open opens a URL in the default browser.
func openBrowser(url string) error {
	args := append(util.GetOSArgsCmd(), url)
	if err := util.ExecuteCmd(args...); err != nil {
		return fmt.Errorf("%w: opening in browser", err)
	}

	return nil
}

// filterSlice select which item to remove from a slice using the
// text editor.
func filterSlice(bs *Slice) error {
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

// bookmarkEdition edits a bookmark with a text editor.
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

// openQR opens a QR-Code image in the system default image
// viewer.
func openQR(qrcode *qr.QRCode, b *Bookmark) error {
	const maxLabelLen = 55
	var title string
	var url string

	if err := qrcode.GenImg(App.GetName()); err != nil {
		return fmt.Errorf("%w", err)
	}

	title = format.ShortenString(b.GetTitle(), maxLabelLen)
	if err := qrcode.Label(title, "top"); err != nil {
		return fmt.Errorf("%w: adding top label", err)
	}

	url = format.ShortenString(b.GetURL(), maxLabelLen)
	if err := qrcode.Label(url, "bottom"); err != nil {
		return fmt.Errorf("%w: adding bottom label", err)
	}

	if err := qrcode.Open(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// confirmEditOrSave confirms if the user wants to save the
// bookmark.
func confirmEditOrSave(b *Bookmark) error {
	save := color.Green("\nsave").Bold().String() + " bookmark?"
	opt := terminal.ConfirmOrEdit(save, []string{"yes", "no", "edit"}, "y")

	switch opt {
	case "n":
		return fmt.Errorf("%w", ErrActionAborted)
	case "e":
		if err := bookmarkEdition(b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// confirmAction prompts the user to confirm the action.
func confirmAction(bs *Slice, prompt string, colors color.ColorFn) error {
	for !Force {
		var summary string

		n := bs.Len()
		if n == 0 {
			return repo.ErrRecordNotFound
		}

		bs.ForEach(func(b Bookmark) {
			summary += format.ColorWithURLPath(&b, terminal.MaxWidth, colors) + "\n"
		})

		summary += prompt + fmt.Sprintf(" %d bookmark/s?", n)
		opt := terminal.ConfirmOrEdit(summary, []string{"yes", "no", "edit"}, "n")

		switch opt {
		case "n", "no":
			return ErrActionAborted
		case "y", "yes":
			Force = true
		case "e", "edit":
			if err := filterSlice(bs); err != nil {
				return err
			}
			terminal.Clear()
		}
	}

	if bs.Len() == 0 {
		return repo.ErrRecordNotFound
	}

	return nil
}

// validateRemove checks if the remove operation is valid.
func validateRemove(bs *Slice) error {
	if bs.Len() == 0 {
		return repo.ErrRecordNotFound
	}

	if terminal.Piped && !Force {
		return fmt.Errorf(
			"%w: input from pipe is not supported yet. use --force",
			ErrActionAborted,
		)
	}

	return nil
}

// removeRecords removes the records from the database.
func removeRecords(r *Repo, bs *Slice) error {
	s := spinner.New()
	s.Mesg = color.Gray("removing record/s...").String()
	s.Start()

	if err := r.DeleteAndReorder(bs, r.Cfg.GetTableMain(), r.Cfg.GetTableDeleted()); err != nil {
		return fmt.Errorf("deleting and reordering records: %w", err)
	}

	s.Stop()

	fmt.Println("bookmark/s removed", color.Green("successfully").Bold())

	return nil
}
