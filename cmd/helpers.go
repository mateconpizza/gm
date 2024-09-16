package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/bookmark/qr"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

var ErrCopyToClipboard = errors.New("copy to clipboard")

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
		fmt.Fprintf(os.Stderr, "%s: %s\n", config.App.Name, err)
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

// filterSlice select which item to remove from a slice using the
// text editor.
func filterSlice(bs *Slice) error {
	buf := bookmark.BufferSlice(bs)
	format.BufferApendVersion(config.App.Name, config.App.Version, &buf)

	editor, err := files.Editor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := files.Edit(editor, &buf); err != nil {
		return fmt.Errorf("on editing slice buffer: %w", err)
	}

	c := format.ByteSliceToLines(&buf)
	urls := bookmark.ExtractContentLine(&c)
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
	buf := bookmark.Buffer(b)
	editor, err := files.Editor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := files.Edit(editor, &buf); err != nil {
		if errors.Is(err, files.ErrBufferUnchanged) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	content := format.ByteSliceToLines(&buf)
	tempB := bookmark.ParseContent(&content)
	tempB = bookmark.ScrapeAndUpdate(tempB)
	if err := bookmark.BufferValidate(&content); err != nil {
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

	if err := qrcode.GenImg(config.App.Name); err != nil {
		return fmt.Errorf("%w", err)
	}

	title = format.Shorten(b.Title, maxLabelLen)
	if err := qrcode.Label(title, "top"); err != nil {
		return fmt.Errorf("%w: adding top label", err)
	}

	url = format.Shorten(b.URL, maxLabelLen)
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
	save := color.BrightGreen("\nsave").Bold().String() + " bookmark?"
	opt := terminal.ConfirmWithOpts(save, []string{"yes", "no", "edit"}, "y")

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

		f := frame.New(frame.WithColorBorder(color.Gray))

		bs.ForEachIdx(func(i int, b Bookmark) {
			bookmark.WithFrameAndColorRenameMe(f, &b, terminal.MinWidth, colors)
		})

		f.Render()

		summary += prompt + fmt.Sprintf(" %d bookmark/s?", n)
		opt := terminal.ConfirmWithOpts(summary, []string{"yes", "no", "edit"}, "n")

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

	if terminal.IsPiped() && !Force {
		return fmt.Errorf(
			"%w: input from pipe is not supported yet. use --force",
			ErrActionAborted,
		)
	}

	return nil
}

// removeRecords removes the records from the database.
func removeRecords(r *repo.SQLiteRepository, bs *Slice) error {
	mesg := color.Gray("removing record/s...").String()
	s := spinner.New(spinner.WithMesg(mesg))
	s.Start()

	if err := r.DeleteAndReorder(bs, r.Cfg.TableMain, r.Cfg.TableDeleted); err != nil {
		return fmt.Errorf("deleting and reordering records: %w", err)
	}

	s.Stop()

	success := color.BrightGreen("Successfully").Italic().Bold()
	fmt.Printf("\n%s bookmark/s removed\n", success)

	return nil
}
