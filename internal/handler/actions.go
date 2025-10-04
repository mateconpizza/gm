// Package handler handles parsing and processing of bookmark data operations.
package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/bookmark/status"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/scraper"
)

var (
	cbc = func(s string) string { return color.BrightCyan(s).Italic().String() } // BrightCyan Italic
	cbb = func(s string) string { return color.BrightBlue(s).Italic().String() } // BrightBlue Italic
	cbg = func(s string) string { return color.BrightGreen(s).Bold().String() }  // BrightGreen Bold
	cy  = func(s string) string { return color.Yellow(s).String() }              // Yellow
	ctb = func(s string) string { return color.Text(s).Bold().String() }         // Bold
	cd  = func(s string) string { return color.Text(s).Dim().String() }          // Dim
)

// QR handles creation, rendering or opening of QR-Codes.
func QR(bs []*bookmark.Bookmark, open bool, appName string) error {
	qrFn := func(b *bookmark.Bookmark) error {
		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		if open {
			return openQR(qrcode, b, appName)
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
func Edit(c *ui.Console, r *db.SQLite, app *config.Config, bs []*bookmark.Bookmark) error {
	const maxItems = 10

	te, err := editor.NewEditor(app.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	q := fmt.Sprintf("%s %d bookmarks", cbg("edit"), len(bs))
	if err := confirmUserLimit(c, len(bs), maxItems, q); err != nil {
		return err
	}

	var s editor.EditStrategy
	switch {
	case app.Flags.Notes:
		s = editor.NotesStrategy{}
	case app.Flags.JSON:
		s = editor.JSONStrategy{}
	default:
		s = editor.BookmarkStrategy{}
	}

	session := editor.NewEditSession(c, te, r,
		editor.WithPostEditionRun(func(o, u *editor.Record) error {
			return git.UpdateBookmark(app, o, u)
		}),
	)

	return session.Run(bs, s)
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

		if err := r.UpdateOne(context.Background(), b); err != nil {
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
			if err := r.UpdateOne(ctx, b); err != nil {
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
	if err := locker.IsLocked(rToLock); err != nil {
		return fmt.Errorf("%w", err)
	}

	if !files.Exists(rToLock) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, filepath.Base(rToLock))
	}

	if err := c.ConfirmErr(fmt.Sprintf("Lock %q?", filepath.Base(rToLock)), "y"); err != nil {
		if errors.Is(err, sys.ErrActionAborted) {
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
func Update(c *ui.Console, r *db.SQLite, app *config.Config, bs []*bookmark.Bookmark) error {
	n := len(bs)
	if n > 1 {
		c.F.Reset().Headerln(cy(fmt.Sprintf("Updating %d bookmarks", n))).Rowln().Flush()
	}

	te, err := editor.NewEditor(app.Env.Editor)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	session := editor.NewEditSession(c, te, r,
		editor.WithPostEditionRun(func(o, u *editor.Record) error {
			return git.UpdateBookmark(app, o, u)
		}),
	)

	for i, b := range bs {
		updated, err := updateBookmarkData(c, b)
		if err != nil {
			return err
		}

		su := txt.Shorten(updated.URL, 60)
		bid := color.Text(fmt.Sprintf("[%d]", b.ID)).Bold().String()
		// Check if there are any changes
		if bytes.Equal(b.Buffer(), updated.Buffer()) {
			fmt.Print(c.Info(bid + " " + cd(su) + " no changes found\n"))
			continue
		}
		// Display changes
		c.F.Reset().Warning(bid + " Found changes in " + cbb(su) + "\n").Flush()
		if !bytes.Equal([]byte(b.Title), []byte(updated.Title)) {
			c.F.Reset().Midln(cbc("Title:")).Flush()
			fmt.Println(txt.DiffColor(txt.Diff([]byte(b.Title), []byte(updated.Title))))
		}
		if !bytes.Equal([]byte(b.Desc), []byte(updated.Desc)) {
			c.F.Reset().Midln(cbc("Description:")).Flush()
			fmt.Println(txt.DiffColor(txt.Diff([]byte(b.Desc), []byte(updated.Desc))))
		}

		// Handle user choice
		opt, err := c.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
		if err != nil {
			return fmt.Errorf("choose: %w", err)
		}
		switch strings.ToLower(opt) {
		case "y", "yes":
			if err := r.UpdateOne(context.Background(), &updated); err != nil {
				return fmt.Errorf("updating record: %w", err)
			}
			if i != n-1 {
				fmt.Println()
			}
		case "n", "no":
			c.ReplaceLine(c.F.Warning(bid + " " + cd(su) + " skipping update\n").String())
		case "e", "edit":
			if err := session.Run([]*bookmark.Bookmark{&updated}, editor.BookmarkStrategy{}); err != nil {
				return err
			}
			fmt.Print(c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", updated.ID)))
		}
	}

	return nil
}

func Export(bs []*bookmark.Bookmark) error {
	return bookio.ExportToNetscapeHTML(bs, os.Stdout)
}

// openQR opens a QR-Code image in the system default image viewer.
func openQR(qrcode *qr.QRCode, b *bookmark.Bookmark, appName string) error {
	const maxLabelLen = 55

	if err := qrcode.GenerateImg(appName); err != nil {
		return fmt.Errorf("%w", err)
	}

	trunc := func(s string) string { return txt.Shorten(s, maxLabelLen) }
	if err := qrcode.Label(trunc(b.Title), "top"); err != nil {
		return fmt.Errorf("%w: adding top label", err)
	}
	if err := qrcode.Label(trunc(b.URL), "bottom"); err != nil {
		return fmt.Errorf("%w: adding bottom label", err)
	}

	return qrcode.Open()
}

// SaveNewBookmark asks the user if they want to save the bookmark.
func SaveNewBookmark(c *ui.Console, r *db.SQLite, b *bookmark.Bookmark, a *config.Config) error {
	if a.Flags.Force {
		return r.InsertMany(context.Background(), []*bookmark.Bookmark{b})
	}

	opt, err := c.Choose("save bookmark?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	switch strings.ToLower(opt) {
	case "n", "no":
		return fmt.Errorf("%w", sys.ErrActionAborted)
	case "e", "edit":
		te, err := editor.NewEditor(a.Env.Editor)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := editor.NewEditSession(c, te, r).Run([]*bookmark.Bookmark{b}, editor.NewBookmarkStrategy{}); err != nil {
			return err
		}
	default:
		if _, err := r.InsertOne(context.Background(), b); err != nil {
			return fmt.Errorf("%w", err)
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
