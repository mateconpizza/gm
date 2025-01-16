package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/bookmark/qr"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
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
	format.BufferAppendVersion(config.App.Name, config.App.Version, &buf)

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
	te, err := files.Editor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := bookmark.Edit(te, bookmark.Buffer(b), b); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// openQR opens a QR-Code image in the system default image viewer.
func openQR(qrcode *qr.QRCode, b *Bookmark) error {
	const maxLabelLen = 55
	var title string
	var burl string

	if err := qrcode.GenImg(config.App.Name); err != nil {
		return fmt.Errorf("%w", err)
	}

	title = format.Shorten(b.Title, maxLabelLen)
	if err := qrcode.Label(title, "top"); err != nil {
		return fmt.Errorf("%w: adding top label", err)
	}

	burl = format.Shorten(b.URL, maxLabelLen)
	if err := qrcode.Label(burl, "bottom"); err != nil {
		return fmt.Errorf("%w: adding bottom label", err)
	}

	if err := qrcode.Open(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// confirmAction prompts the user to confirm the action.
func confirmAction(bs *Slice, prompt string, colors color.ColorFn) error {
	for !Force {
		n := bs.Len()
		if n == 0 {
			return repo.ErrRecordNotFound
		}

		bs.ForEachIdx(func(i int, b Bookmark) {
			fmt.Println(bookmark.FrameFormatted(&b, terminal.MinWidth, colors))
		})

		// render frame
		f := frame.New(frame.WithColorBorder(colors), frame.WithNoNewLine())
		q := f.Footer(prompt + fmt.Sprintf(" %d bookmark/s?", n)).String()
		opt := terminal.ConfirmWithChoices(q, []string{"yes", "no", "edit"}, "n")
		opt = strings.ToLower(opt)
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

	if bs.Empty() {
		return repo.ErrRecordNotFound
	}

	return nil
}

// validateRemove checks if the remove operation is valid.
func validateRemove(bs *Slice) error {
	if bs.Empty() {
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

	if err := r.DeleteAndReorder(bs, r.Cfg.Tables.Main, r.Cfg.Tables.Deleted); err != nil {
		return fmt.Errorf("deleting and reordering records: %w", err)
	}

	s.Stop()

	terminal.ClearLine(1)
	success := color.BrightGreen("Successfully").Italic().String()
	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Success(success + " bookmark/s removed").Render()

	return nil
}

func validURL(s string) bool {
	parsedUrl, err := url.Parse(s)
	if err != nil {
		return false
	}

	return parsedUrl.Scheme != "" && parsedUrl.Host != ""
}

// handleMenuOpts returns the options for the menu.
func handleMenuOpts() []menu.OptFn {
	// menu opts
	opts := []menu.OptFn{
		menu.WithDefaultKeybinds(),
		menu.WithDefaultSettings(),
		menu.WithMultiSelection(),
	}

	if !subCommandCalled {
		opts = append(opts,
			menu.WithPreview(),
			menu.WithKeybindEdit(),
			menu.WithKeybindOpen(),
			menu.WithKeybindQR(),
		)
	}

	return opts
}

// confirmUserLimit prompts the user to confirm the exceeding limit.
func confirmUserLimit(count, maxItems int, q string) error {
	if Force || count < maxItems {
		return nil
	}
	defer terminal.ClearLine(1)
	f := frame.New(frame.WithColorBorder(color.BrightBlue), frame.WithNoNewLine()).Header(q)
	if !terminal.Confirm(f.String(), "n") {
		return ErrActionAborted
	}

	return nil
}

// copyBookmarks copies the URL of the first bookmark in the provided Slice to
// the clipboard.
func copyBookmarks(bs *Slice) error {
	if !Copy {
		return nil
	}

	if err := sys.CopyClipboard(bs.Item(0).URL); err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	return nil
}

// openBookmarks opens the URLs in the browser for the bookmarks in the provided Slice.
func openBookmarks(bs *Slice) error {
	// FIX: keep it simple
	if !Open {
		return nil
	}

	const maxGoroutines = 15
	// get user confirmation to procced
	o := color.BrightGreen("opening").Bold()
	s := fmt.Sprintf("%s %d bookmarks, continue?", o, bs.Len())
	if err := confirmUserLimit(bs.Len(), maxGoroutines, s); err != nil {
		return err
	}

	sp := spinner.New(spinner.WithMesg(color.BrightGreen("opening bookmarks...").String()))
	defer sp.Stop()
	sp.Start()

	sem := semaphore.NewWeighted(maxGoroutines)
	var wg sync.WaitGroup
	errCh := make(chan error, bs.Len())
	action := func(b Bookmark) error {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}
		defer sem.Release(1)

		wg.Add(1)
		go func(b Bookmark) {
			defer wg.Done()
			if err := sys.OpenInBrowser(b.URL); err != nil {
				errCh <- fmt.Errorf("open error: %w", err)
			}
		}(b)

		return nil
	}

	if err := bs.ForEachErr(action); err != nil {
		return fmt.Errorf("%w", err)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}

	return nil
}
