package cmd

import (
	"context"
	"errors"
	"fmt"
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

var (
	ErrActionAborted = errors.New("action aborted")
	ErrInvalidOption = errors.New("invalid option")
)

// handleByField prints the selected field.
func handleByField(bs *Slice) error {
	if Field == "" {
		return nil
	}

	Exit = true

	printer := func(b Bookmark) error {
		f, err := b.Field(Field)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(f)

		return nil
	}

	if err := bs.ForEachErr(printer); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handlePrintOut prints the bookmarks in different formats.
func handlePrintOut(bs *Slice) error {
	if Exit {
		return nil
	}

	n := terminal.MinWidth
	lastIdx := bs.Len() - 1

	bs.ForEachIdx(func(i int, b Bookmark) {
		fmt.Print(bookmark.Frame(&b, n))
		if i != lastIdx {
			fmt.Println()
		}
	})

	return nil
}

// handleOneline formats the bookmarks in oneline.
func handleOneline(bs *Slice) error {
	if !Oneline {
		return nil
	}

	Exit = true

	bs.ForEach(func(b Bookmark) {
		fmt.Print(bookmark.Oneline(&b, terminal.MaxWidth))
	})

	return nil
}

// handleJSONFormat formats the bookmarks in JSON.
func handleJSONFormat(bs *Slice) error {
	if !JSON {
		return nil
	}

	Exit = true

	if bs.Len() == 0 {
		fmt.Println(string(format.ToJSON(config.App)))
		return nil
	}

	fmt.Println(string(format.ToJSON(bs.Items())))

	return nil
}

// handleHeadAndTail returns a slice of bookmarks with limited elements.
func handleHeadAndTail(bs *Slice) error {
	if Head == 0 && Tail == 0 {
		return nil
	}

	if Head < 0 || Tail < 0 {
		return fmt.Errorf("%w: head=%d tail=%d", ErrInvalidOption, Head, Tail)
	}

	bs.Head(Head)
	bs.Tail(Tail)

	return nil
}

// handleByQuery executes a search query on the given repository based on
// provided arguments.
func handleByQuery(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	if bs.Len() != 0 || len(args) == 0 {
		return nil
	}

	query := strings.Join(args, "%")
	if err := r.ByQuery(r.Cfg.TableMain, query, bs); err != nil {
		return fmt.Errorf("%w: '%s'", err, strings.Join(args, " "))
	}

	return nil
}

// handleByTags returns a slice of bookmarks based on the provided tags.
func handleByTags(r *repo.SQLiteRepository, bs *Slice) error {
	if Tags == nil {
		return nil
	}

	// TODO: if the slice contains bookmarks, filter by tag.
	if bs.Len() != 0 {
		for _, tag := range Tags {
			bs.Filter(func(b Bookmark) bool {
				return strings.Contains(b.Tags, tag)
			})
		}

		return nil
	}

	for _, tag := range Tags {
		if err := r.ByTag(r.Cfg.TableMain, tag, bs); err != nil {
			return fmt.Errorf("byTags :%w", err)
		}
	}

	if bs.Len() == 0 {
		t := strings.Join(Tags, ", ")
		return fmt.Errorf("%w by tag: '%s'", repo.ErrRecordNoMatch, t)
	}

	bs.Filter(func(b Bookmark) bool {
		for _, tag := range Tags {
			if !strings.Contains(b.Tags, tag) {
				return false
			}
		}

		return true
	})

	return nil
}

// handleEdition renders the edition interface.
func handleEdition(r *repo.SQLiteRepository, bs *Slice) error {
	if !Edit {
		return nil
	}

	Exit = true

	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	const maxItems = 10
	e := color.BrightOrange("editing").Bold()
	q := fmt.Sprintf("%s %d bookmarks, continue?", e, n)
	if err := confirmUserLimit(n, maxItems, q); err != nil {
		return err
	}

	header := "# [%d/%d] | %d | %s\n\n"
	te, err := files.Editor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// edition edits the bookmark with a text editor.
	edition := func(i int, b Bookmark) error {
		// prepare header and buffer
		buf := bookmark.Buffer(&b)
		tShort := format.Shorten(b.Title, terminal.MinWidth-10)
		format.BufferAppend(fmt.Sprintf(header, i+1, n, b.ID, tShort), &buf)
		format.BufferApendVersion(config.App.Name, config.App.Version, &buf)
		bufCopy := make([]byte, len(buf))
		copy(bufCopy, buf)

		if err := bookmark.Edit(te, buf, &b); err != nil {
			if errors.Is(err, bookmark.ErrBufferUnchanged) {
				return nil
			}

			return fmt.Errorf("%w", err)
		}

		if _, err := r.Update(r.Cfg.TableMain, &b); err != nil {
			return fmt.Errorf("handle edition: %w", err)
		}

		fmt.Printf("%s: [%d] %s\n", config.App.Name, b.ID, color.Blue("updated").Bold())

		return nil
	}

	if err := bs.ForEachErrIdx(edition); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleRemove prompts the user the records to remove.
func handleRemove(r *repo.SQLiteRepository, bs *Slice) error {
	if !Remove {
		return nil
	}

	Exit = true

	if err := validateRemove(bs); err != nil {
		return err
	}

	prompt := color.BrightRed("remove").Bold().String()
	if err := confirmAction(bs, prompt, color.BrightRed); err != nil {
		return err
	}

	return removeRecords(r, bs)
}

// handleCheckStatus prints the status code of the bookmark URL.
func handleCheckStatus(bs *Slice) error {
	if !Status {
		return nil
	}

	Exit = true

	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	const maxGoroutines = 15
	status := color.BrightGreen("status").Bold()
	q := fmt.Sprintf("checking %s of %d, continue?", status, n)
	if err := confirmUserLimit(n, maxGoroutines, q); err != nil {
		return ErrActionAborted
	}

	f := frame.New(frame.WithColorBorder(color.BrightBlue))
	f.Header(fmt.Sprintf("checking %s of %d bookmarks", status, n))
	f.Render()
	if err := bookmark.Status(bs); err != nil {
		return fmt.Errorf("%w", err)
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

// handleCopyOpen calls the copyBookmarks and openBookmarks functions, and handles the overall logic.
func handleCopyOpen(bs *Slice) error {
	if Exit {
		return nil
	}

	if !Copy && !Open {
		return nil
	}

	if err := copyBookmarks(bs); err != nil {
		return err
	}

	if err := openBookmarks(bs); err != nil {
		return err
	}

	Exit = Copy || Open

	return nil
}

// handleBookmarksFromArgs retrieves records from the database based on either
// an ID or a query string.
func handleIDsFromArgs(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	ids, err := extractIDsFromStr(args)
	if len(ids) == 0 {
		return nil
	}

	if !errors.Is(err, bookmark.ErrInvalidID) && err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := r.ByIDList(r.Cfg.TableMain, ids, bs); err != nil {
		return fmt.Errorf("records from args: %w", err)
	}

	if bs.Len() == 0 {
		a := strings.TrimRight(strings.Join(args, " "), "\n")
		return fmt.Errorf("%w by id/s: %s", repo.ErrRecordNotFound, a)
	}

	return nil
}

// handleQR handles creation, rendering or opening of QR-Codes.
func handleQR(bs *Slice) error {
	if !QR {
		return nil
	}

	Exit = true

	qrFn := func(b Bookmark) error {
		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		if Open {
			return openQR(qrcode, &b)
		}

		var sb strings.Builder
		sb.WriteString(b.Title + "\n")
		sb.WriteString(b.URL)
		sb.WriteString(qrcode.String())
		t := sb.String()
		fmt.Print(t)

		lines := len(strings.Split(t, "\n"))
		terminal.WaitForEnter()
		terminal.ClearLine(lines)

		return nil
	}

	if err := bs.ForEachErr(qrFn); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleMenu allows the user to select multiple records.
func handleMenu(bs *Slice) error {
	if !Menu {
		return nil
	}
	if bs.Len() == 0 {
		return repo.ErrRecordNoMatch
	}

	if err := menu.LoadConfig(); err != nil {
		return fmt.Errorf("%w", err)
	}

	menu.WithColor(&config.App.Color)

	// menu opts
	opts := []menu.OptFn{
		menu.WithDefaultKeybinds(),
		menu.WithDefaultSettings(),
		menu.WithKeybindEdit(),
		menu.WithKeybindOpen(),
		menu.WithKeybindQR(),
		menu.WithPreview(),
		menu.WithMultiSelection(),
	}

	var formatter func(*Bookmark, int) string
	if Multiline {
		opts = append(opts, menu.WithMultilineView())
		formatter = bookmark.Multiline
	} else {
		formatter = bookmark.Oneline
	}

	m := menu.New[Bookmark](opts...)
	var result []Bookmark
	result, err := m.Select(bs.Items(), func(b Bookmark) string {
		return formatter(&b, terminal.MaxWidth)
	})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(result) == 0 {
		return nil
	}

	bs.Set(&result)

	return nil
}
