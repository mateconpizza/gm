package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/bookmark/scraper"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

var (
	cbc = func(s string) string { return color.BrightCyan(s).Italic().String() }
	cbb = func(s string) string { return color.BrightBlue(s).Italic().String() }
	cbg = func(s string) string { return color.BrightGreen(s).Bold().String() }
	cy  = func(s string) string { return color.Yellow(s).String() }
)

// QR handles creation, rendering or opening of QR-Codes.
func QR(bs *slice.Slice[bookmark.Bookmark], open bool) error {
	qrFn := func(b bookmark.Bookmark) error {
		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		if open {
			return openQR(qrcode, &b)
		}

		var sb strings.Builder

		sb.WriteString(b.Title + "\n")
		sb.WriteString(b.URL + "\n")
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

// Copy copies the URLs to the system clipboard.
func Copy(bs *slice.Slice[bookmark.Bookmark]) error {
	var urls string

	bs.ForEach(func(b bookmark.Bookmark) {
		urls += b.URL + "\n"
	})

	if err := sys.CopyClipboard(urls); err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	return nil
}

// Open opens the URLs in the browser for the bookmarks in the provided Slice.
func Open(c *ui.Console, r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	const maxGoroutines = 15
	// get user confirmation to procced
	s := fmt.Sprintf("%s %d bookmarks, continue?", cbg("opening"), bs.Len())
	if err := confirmUserLimit(c, bs.Len(), maxGoroutines, s); err != nil {
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
		errCh = make(chan error, bs.Len())
	)

	actionFn := func(b bookmark.Bookmark) error {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}
		defer sem.Release(1)

		wg.Add(1)

		go func(b bookmark.Bookmark) {
			defer wg.Done()

			if err := sys.OpenInBrowser(b.URL); err != nil {
				errCh <- fmt.Errorf("open error: %w", err)
			}
		}(b)

		return nil
	}

	if err := bs.ForEachErr(actionFn); err != nil {
		return fmt.Errorf("%w", err)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}

	updateVisit := func(b bookmark.Bookmark) error {
		return r.UpdateVisitDateAndCount(context.Background(), &b)
	}
	if err := bs.ForEachErr(updateVisit); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// CheckStatus prints the status code of the bookmark URL.
func CheckStatus(c *ui.Console, bs *slice.Slice[bookmark.Bookmark]) error {
	n := bs.Len()
	if n == 0 {
		return db.ErrRecordQueryNotProvided
	}

	const maxGoroutines = 15

	q := fmt.Sprintf("checking %s of %d, continue?", cbg("status"), n)
	if err := confirmUserLimit(c, n, maxGoroutines, q); err != nil {
		return sys.ErrActionAborted
	}

	c.F.Header(fmt.Sprintf("checking %s of %d bookmarks\n", cbg("status"), n)).Flush()

	if err := bookmark.Status(bs); err != nil {
		return fmt.Errorf("%w", err)
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
		return locker.ErrItemUnlocked
	}

	if !strings.HasSuffix(rToUnlock, ".enc") {
		rToUnlock += ".enc"
	}

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

// EditBookmarks edits a slice of bookmarks.
func EditBookmarks(
	c *ui.Console,
	r *db.SQLiteRepository,
	te *files.TextEditor,
	bs []*bookmark.Bookmark,
) error {
	n := len(bs)
	if n == 0 {
		return bookmark.ErrNotFound
	}

	for i := range bs {
		if err := editSingleInteractive(c, r, te, bs[i], i, n); err != nil {
			return err
		}
	}

	return nil
}

// editSingleInteractive handles editing a single bookmark with confirmation and retry.
func editSingleInteractive(
	c *ui.Console,
	r *db.SQLiteRepository,
	te *files.TextEditor,
	b *bookmark.Bookmark,
	index, total int,
) error {
	current := *b

	for {
		editedB, err := bookmark.Edit(te, &current, index, total)
		if err != nil {
			if errors.Is(err, bookmark.ErrBufferUnchanged) {
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
		//nolint:goconst //ignore
		case "n", "no":
			return sys.ErrActionAborted
		case "e", "edit":
			current = *editedB
		}
	}
}

// EditSlice edits the bookmarks using a text editor.
func EditSlice(c *ui.Console, r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	te, err := files.NewEditor(config.App.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := EditBookmarks(c, r, te, bs.ItemsPtr()); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// SaveNewBookmark asks the user if they want to save the bookmark.
func SaveNewBookmark(c *ui.Console, r *db.SQLiteRepository, b *bookmark.Bookmark) error {
	if config.App.Flags.Force {
		if err := r.InsertOne(context.Background(), b); err != nil {
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

		if err := EditBookmarks(c, r, te, []*bookmark.Bookmark{b}); err != nil {
			return err
		}
	default:
		if err := r.InsertOne(context.Background(), b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// UpdateSlice updates the bookmarks.
//
// It uses the scraper to update the title and description.
func UpdateSlice(c *ui.Console, r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	c.F.Reset().Headerln(cy(fmt.Sprintf("Updating %d bookmark/s", bs.Len()))).Rowln().Flush()

	updateFn := func(i int, b bookmark.Bookmark) error {
		updatedB := b
		bid := cbc(fmt.Sprintf("[%d]", b.ID))
		su := txt.Shorten(updatedB.URL, 60)

		sc := scraper.New(updatedB.URL, scraper.WithSpinner(c.Info("updating bookmark "+cbc(su)).String()))
		if err := sc.Start(); err != nil {
			slog.Error("scraping error", "error", err)
		}

		updatedB.Title, _ = sc.Title()
		updatedB.Desc, _ = sc.Desc()

		if bytes.Equal(b.Buffer(), updatedB.Buffer()) {
			fmt.Print(c.Info(bid + " " + cbb(su) + " no changes detected\n"))
			return nil
		}

		c.F.Reset().Warning("Found changes in " + bid + " " + cbb(su) + "\n").Flush()

		if !bytes.Equal([]byte(b.Title), []byte(updatedB.Title)) {
			c.F.Reset().Midln(cbc("Title:")).Flush()
			fmt.Println(txt.DiffColor(txt.Diff([]byte(b.Title), []byte(updatedB.Title))))
		}

		if !bytes.Equal([]byte(b.Desc), []byte(updatedB.Desc)) {
			c.F.Reset().Midln(cbc("Description:")).Flush()
			fmt.Println(txt.DiffColor(txt.Diff([]byte(b.Desc), []byte(updatedB.Desc))))
		}

		opt, err := c.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
		if err != nil {
			return fmt.Errorf("choose: %w", err)
		}

		switch strings.ToLower(opt) {
		case "y", "yes":
			return handleEditedBookmark(c, r, &updatedB, &b)
		case "n", "no":
			return sys.ErrActionAborted
		case "e", "edit":
			te, err := files.NewEditor(config.App.Env.Editor)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return editSingleInteractive(c, r, te, &updatedB, i, bs.Len())
		}

		return nil
	}

	if err := bs.ForEachIdxErr(updateFn); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
