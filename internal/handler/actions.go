package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/bookmark/status"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/parser"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/scraper"
)

var (
	cbc = func(s string) string { return color.BrightCyan(s).Italic().String() }
	cbb = func(s string) string { return color.BrightBlue(s).Italic().String() }
	cbg = func(s string) string { return color.BrightGreen(s).Bold().String() }
	cy  = func(s string) string { return color.Yellow(s).String() }
	ctb = func(s string) string { return color.Text(s).Bold().String() }
)

// QR handles creation, rendering or opening of QR-Codes.
func QR(bs []*bookmark.Bookmark, open bool) error {
	qrFn := func(b *bookmark.Bookmark) error {
		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		if open {
			return openQR(qrcode, b)
		}

		var sb strings.Builder
		sb.WriteString(b.Title + "\n")
		sb.WriteString(b.URL + "\n")
		sb.WriteString(qrcode.String())
		fmt.Print(sb.String())

		terminal.WaitForEnter()
		terminal.ClearLine(txt.CountLines(sb.String()))

		return nil
	}

	for i := range bs {
		if err := qrFn(bs[i]); err != nil {
			return err
		}
	}

	return nil
}

// Copy copies the URLs to the system clipboard.
func Copy(bs []*bookmark.Bookmark) error {
	var urls string
	for i := range bs {
		urls += bs[i].URL + "\n"
	}
	if err := sys.CopyClipboard(urls); err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	return nil
}

// Open opens the URLs in the browser for the bookmarks in the provided Slice.
func Open(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) error {
	const maxGoroutines = 15
	n := len(bs)
	// get user confirmation to procced
	s := fmt.Sprintf("%s %d bookmarks", cbg("open"), n)
	if err := confirmUserLimit(c, n, maxGoroutines, s); err != nil {
		return err
	}

	sp := rotato.New(
		rotato.WithMesg("opening bookmarks..."),
		rotato.WithMesgColor(rotato.ColorBrightGreen),
		rotato.WithSpinnerColor(rotato.ColorBrightGreen),
	)
	sp.Start()
	defer sp.Done()

	var (
		wg    sync.WaitGroup
		sem   = semaphore.NewWeighted(maxGoroutines)
		errCh = make(chan error, n)
		ctx   = context.Background()
	)

	actionFn := func(b *bookmark.Bookmark) error {
		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}
		defer sem.Release(1)
		wg.Add(1)

		go func(b *bookmark.Bookmark) {
			defer wg.Done()

			if err := sys.OpenInBrowser(b.URL); err != nil {
				errCh <- fmt.Errorf("open error: %w", err)
			}
		}(b)

		return nil
	}

	for _, b := range bs {
		if err := actionFn(b); err != nil {
			return err
		}
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}

	for _, b := range bs {
		if err := r.AddVisit(ctx, b.ID); err != nil {
			return err
		}
	}

	return nil
}

// Edit edits the bookmarks using a text editor.
func Edit(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) error {
	te, err := files.NewEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if config.App.Flags.JSON {
		return editBookmarksJSON(c, r, te, bs)
	}

	return editBookmarks(c, r, te, bs)
}

// CheckStatus prints the status code of the bookmark URL.
func CheckStatus(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) error {
	const maxGoroutines = 15

	n := len(bs)
	if n == 0 {
		return db.ErrRecordQueryNotProvided
	}

	s := fmt.Sprintf("checking status of %d bookmarks", n)
	if err := confirmUserLimit(c, n, maxGoroutines, s); err != nil {
		return sys.ErrActionAborted
	}

	if err := status.Check(c, bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	for i := range bs {
		b := bs[i]
		if b.HTTPStatusCode == http.StatusTooManyRequests {
			continue
		}

		if err := r.Update(context.Background(), b, b); err != nil {
			return err
		}
	}

	return nil
}

//nolint:funlen //i
func Snapshot(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) error {
	const maxConRequests = 10
	var (
		count   uint32
		sem     = semaphore.NewWeighted(int64(maxConRequests))
		ctx     = context.Background()
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    = make(chan string, len(bs))
		success = make(chan string, len(bs))
	)

	sp := rotato.New(
		rotato.WithPrefix(c.F.Mid("Fetching snapshots").String()),
		rotato.WithMesgColor(rotato.ColorYellow),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
		rotato.WithFailColorMesg(rotato.ColorBrightRed),
	)
	sp.Start()

	n := len(bs)
	for _, b := range bs {
		if b.ArchiveURL != "" && b.ArchiveTimestamp != "" {
			continue
		}

		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("acquire semaphore: %w", err)
		}

		wg.Add(1)

		go func() {
			defer wg.Done()
			defer sem.Release(1)

			id := fmt.Sprintf("ID %s", color.Text(fmt.Sprintf("%-3d", b.ID)).Bold())
			d := id + " " + txt.Shorten(b.URL, 60)

			f := frame.New(frame.WithColorBorder(color.Gray))
			currentCount := atomic.AddUint32(&count, 1)
			sp.UpdateMesg(fmt.Sprintf("[%d/%d] %s", currentCount, n, f.Info(txt.Shorten(b.URL, 80)).String()))
			f.Reset()

			s, err := scraper.WaybackSnapshot(b.URL)
			if err != nil {
				es := color.BrightGray(" (" + err.Error() + ")").Italic().String()
				errs <- f.Error(d).Text(es).String()
				return
			}

			b.ArchiveURL = s.URL
			b.ArchiveTimestamp = s.Timestamp

			mu.Lock()
			if err := r.Update(ctx, b, b); err != nil {
				mu.Unlock()
				es := color.BrightGray(" (" + err.Error() + ")").Italic().String()
				errs <- f.Error(d).Text(es).String()
				return
			}
			mu.Unlock()

			success <- f.Success(d).String()
		}()
	}

	wg.Wait()
	close(errs)
	close(success)

	sp.Done()

	if count == 0 {
		return nil
	}

	c.F.Reset().Header(ctb(fmt.Sprintf("Summary %d bookmarks\n", count))).Rowln()

	if len(success) > 0 {
		c.F.Midln("Updated").Flush()
		for s := range success {
			c.F.Textln(s).Flush()
		}
		c.F.Rowln()
	}

	if len(errs) > 0 {
		c.F.Midln("Failed").Flush()
		for err := range errs {
			c.F.Textln(err).Flush()
		}
	}

	return nil
}

// LockRepo locks the database.
func LockRepo(c *ui.Console, rToLock string) error {
	slog.Debug("locking database", "name", config.App.DBName)

	if err := locker.IsLocked(rToLock); err != nil {
		return fmt.Errorf("%w", err)
	}

	if !files.Exists(rToLock) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, filepath.Base(rToLock))
	}

	if err := c.ConfirmErr(fmt.Sprintf("Lock %q?", filepath.Base(rToLock)), "y"); err != nil {
		if errors.Is(err, terminal.ErrActionAborted) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}

	pass, err := passwordConfirm(c)
	if err != nil {
		return err
	}

	if err := locker.Lock(rToLock, pass); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("database locked: %q\n", filepath.Base(rToLock))))

	return nil
}

// UnlockRepo unlocks the database.
func UnlockRepo(c *ui.Console, rToUnlock string) error {
	if err := locker.IsLocked(rToUnlock); err == nil {
		return fmt.Errorf("%w: %q", locker.ErrFileUnlocked, filepath.Base(rToUnlock))
	}

	rToUnlock = files.EnsureSuffix(rToUnlock, locker.Extension)
	slog.Debug("unlocking database", "name", rToUnlock)

	if !files.Exists(rToUnlock) {
		s := filepath.Base(strings.TrimSuffix(rToUnlock, ".enc"))
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, s)
	}

	if err := c.ConfirmErr(fmt.Sprintf("Unlock %q?", filepath.Base(rToUnlock)), "y"); err != nil {
		return fmt.Errorf("%w", err)
	}

	s, err := c.InputPassword("Password: ")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := locker.Unlock(rToUnlock, s); err != nil {
		fmt.Println()
		return fmt.Errorf("%w", err)
	}

	fmt.Println()
	fmt.Print(c.SuccessMesg("database unlocked\n"))

	return nil
}

// Update updates the bookmarks.
//
// It uses the scraper to update the title, description and favicon.
func Update(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark) error {
	if len(bs) > 1 {
		c.F.Reset().Headerln(cy(fmt.Sprintf("Updating %d bookmarks", len(bs)))).Rowln().Flush()
	}

	for i, b := range bs {
		if err := updateSingleBookmark(c, r, b, i, len(bs)); err != nil &&
			!errors.Is(err, sys.ErrActionAborted) {
			return err
		}
	}

	return nil
}

// openQR opens a QR-Code image in the system default image viewer.
func openQR(qrcode *qr.QRCode, b *bookmark.Bookmark) error {
	const maxLabelLen = 55

	var title, burl string

	if err := qrcode.GenerateImg(config.App.Name); err != nil {
		return fmt.Errorf("%w", err)
	}

	title = txt.Shorten(b.Title, maxLabelLen)
	if err := qrcode.Label(title, "top"); err != nil {
		return fmt.Errorf("%w: adding top label", err)
	}

	burl = txt.Shorten(b.URL, maxLabelLen)
	if err := qrcode.Label(burl, "bottom"); err != nil {
		return fmt.Errorf("%w: adding bottom label", err)
	}

	if err := qrcode.Open(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// editBookmarks edits a slice of bookmarks.
func editBookmarks(
	c *ui.Console,
	r *db.SQLite,
	te *files.TextEditor,
	bs []*bookmark.Bookmark,
) error {
	n := len(bs)
	if n == 0 {
		return bookmark.ErrBookmarkNotFound
	}

	for i := range bs {
		if err := editSingleInteractive(c, r, te, bs[i], i, n); err != nil {
			return err
		}
	}

	return nil
}

func editBookmarksJSON(
	c *ui.Console,
	r *db.SQLite,
	te *files.TextEditor,
	bs []*bookmark.Bookmark,
) error {
	for i := range bs {
		b := bs[i]
		oldB := b.Bytes()

	out:
		for {
			newB, err := te.EditBytes(oldB, "json")
			if err != nil {
				return err
			}

			oldB = bytes.TrimRight(oldB, "\n")
			newB = bytes.TrimRight(newB, "\n")
			if bytes.Equal(newB, oldB) {
				break out
			}

			diff := txt.Diff(oldB, newB)
			fmt.Println(txt.DiffColor(diff))
			opt, err := c.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
			if err != nil {
				return fmt.Errorf("choose: %w", err)
			}

			switch strings.ToLower(opt) {
			case "y", "yes":
				bm, err := bookmark.NewFromBuffer(newB)
				if err != nil {
					return err
				}

				if err := r.Update(context.Background(), bm, b); err != nil {
					return fmt.Errorf("update: %w", err)
				}

				fmt.Print(c.SuccessMesg("bookmark updated\n"))

				break out
			case "n", "no":
				return sys.ErrActionAborted
			case "e", "edit":
				oldB = newB
			}
		}
	}

	return nil
}

// editSingleInteractive handles editing a single bookmark with confirmation and retry.
func editSingleInteractive(
	c *ui.Console,
	r *db.SQLite,
	te *files.TextEditor,
	b *bookmark.Bookmark,
	index, total int,
) error {
	current := *b

	for {
		editedB, err := parser.Edit(te, &current, index, total)
		if err != nil {
			if errors.Is(err, parser.ErrBufferUnchanged) {
				return nil
			}

			return fmt.Errorf("edit: %w", err)
		}

		c.F.Reset().Header(cy("Edit Bookmark:\n\n")).Flush()

		diff := txt.Diff(current.Buffer(), editedB.Buffer())
		fmt.Println(txt.DiffColor(diff))

		opt, err := c.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
		if err != nil {
			return fmt.Errorf("choose: %w", err)
		}

		switch strings.ToLower(opt) {
		case "y", "yes":
			return handleEditedBookmark(c, r, editedB, b)
		case "n", "no":
			return sys.ErrActionAborted
		case "e", "edit":
			current = *editedB
		}
	}
}

// SaveNewBookmark asks the user if they want to save the bookmark.
func SaveNewBookmark(c *ui.Console, r *db.SQLite, b *bookmark.Bookmark, force bool) error {
	if force {
		if err := r.InsertMany(context.Background(), []*bookmark.Bookmark{b}); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	}

	opt, err := c.Choose("save bookmark?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	switch strings.ToLower(opt) {
	case "n", "no":
		return fmt.Errorf("%w", sys.ErrActionAborted)
	case "e", "edit":
		te, err := files.NewEditor(config.App.Env.Editor)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := editBookmarks(c, r, te, []*bookmark.Bookmark{b}); err != nil {
			return err
		}
	default:
		if _, err := r.InsertOne(context.Background(), b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// updateSingleBookmark processes a single bookmark update including scraping,
// diff display, and user interaction for saving changes.
func updateSingleBookmark(c *ui.Console, r *db.SQLite, b *bookmark.Bookmark, index, total int) error {
	updatedB, err := updateBookmarkData(c, b)
	if err != nil {
		return err
	}

	su := txt.Shorten(updatedB.URL, 60)
	bid := color.Text(fmt.Sprintf("[%d]", b.ID)).Bold().String()

	// Check if there are any changes
	if bytes.Equal(b.Buffer(), updatedB.Buffer()) {
		fmt.Print(c.Info(bid + " " + cbb(su) + " no changes found\n"))
		return nil
	}

	// Display changes
	c.F.Reset().Warning(bid + " Found changes in " + cbb(su) + "\n").Flush()
	if !bytes.Equal([]byte(b.Title), []byte(updatedB.Title)) {
		c.F.Reset().Midln(cbc("Title:")).Flush()
		fmt.Println(txt.DiffColor(txt.Diff([]byte(b.Title), []byte(updatedB.Title))))
	}
	if !bytes.Equal([]byte(b.Desc), []byte(updatedB.Desc)) {
		c.F.Reset().Midln(cbc("Description:")).Flush()
		fmt.Println(txt.DiffColor(txt.Diff([]byte(b.Desc), []byte(updatedB.Desc))))
	}

	// Handle user choice
	opt, err := c.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("choose: %w", err)
	}

	switch strings.ToLower(opt) {
	case "y", "yes":
		if err := handleEditedBookmark(c, r, &updatedB, b); err != nil {
			return err
		}
		if index != total-1 {
			fmt.Println()
		}
	case "n", "no":
		c.ReplaceLine(c.F.Warning(bid + " skip...\n").String())
	case "e", "edit":
		te, err := files.NewEditor(config.App.Env.Editor)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := editSingleInteractive(c, r, te, &updatedB, index, total); err != nil {
			return err
		}
	}

	return nil
}

func updateBookmarkData(c *ui.Console, b *bookmark.Bookmark) (bookmark.Bookmark, error) {
	updatedB := *b
	su := txt.Shorten(updatedB.URL, 60)
	bid := color.Text(fmt.Sprintf("[%d]", b.ID)).Bold().String()

	sc := scraper.New(
		updatedB.URL,
		scraper.WithSpinner(c.Info(bid+" updating bookmark "+cbc(su)).String()),
	)

	if err := sc.Start(); err != nil {
		return updatedB, err
	}

	updatedB.Title, _ = sc.Title()
	updatedB.Desc, _ = sc.Desc()
	updatedB.FaviconURL, _ = sc.Favicon()
	return updatedB, nil
}
