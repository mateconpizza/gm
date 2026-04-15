// Package handler handles parsing and processing of bookmark data operations.
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

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/scraper"
)

// QR handles creation, rendering or opening of QR-Codes.
func QR(a *app.Context, bs []*bookmark.Bookmark) error {
	qrFn := func(b *bookmark.Bookmark) error {
		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		p := a.Console().Palette()
		var sb strings.Builder
		sb.WriteString(p.Bold.Sprint(b.Title + "\n"))
		sb.WriteString(p.Italic.Sprint(b.URL + "\n"))
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

// QROpen opens a QR-Code image in the system default image viewer.
func QROpen(ctx context.Context, qrcode *qr.QRCode, b *bookmark.Bookmark, appName string) error {
	const maxLabelLen = 55

	if err := qrcode.GenerateImg(appName); err != nil {
		return fmt.Errorf("%w", err)
	}

	trunc := func(s string) string { return txt.Shorten(s, maxLabelLen) }
	if err := qrcode.Label(trunc(b.Title), qr.LabelTop); err != nil {
		return fmt.Errorf("%w: adding top label", err)
	}
	if err := qrcode.Label(trunc(b.URL), qr.LabelBottom); err != nil {
		return fmt.Errorf("%w: adding bottom label", err)
	}

	return qrcode.Open(ctx)
}

// Open opens the URLs in the browser for the bookmarks in the provided Slice.
func Open(a *app.Context, bs []*bookmark.Bookmark) error {
	const maxGoroutines = 5
	n := len(bs)
	p := a.Console().Palette()

	// get user confirmation to procced
	s := fmt.Sprintf("%s %d bookmarks", p.BrightGreen.Wrap("open", p.Bold), n)
	if err := a.Console().ConfirmLimit(n, maxGoroutines, s, a.Cfg.Flags.Force); err != nil {
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
	)
	actionFn := func(b *bookmark.Bookmark) error {
		if err := sem.Acquire(a.Context(), 1); err != nil {
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
		if err := a.DB.AddVisit(a.Context(), b.ID); err != nil {
			return err
		}
	}

	return nil
}

// Edit returns a BookmarkAction configured with a specific strategy.
func Edit(es editor.EditStrategy) func(*app.Context, []*bookmark.Bookmark) error {
	return func(a *app.Context, bs []*bookmark.Bookmark) error {
		const maxItems = 10
		p := a.Console().Palette()
		q := fmt.Sprintf("%s %d bookmarks", p.BrightGreen.Wrap("edit", p.Bold), len(bs))

		if err := a.Console().ConfirmLimit(len(bs), maxItems, q, a.Cfg.Flags.Force); err != nil {
			return err
		}

		return runEditSession(a, bs, es,
			editor.WithPostEditionRunE(func(o, u *editor.Record) error {
				return git.UpdateBookmark(a.Cfg, o, u)
			}),
		)
	}
}

// LockRepo locks the database.
func LockRepo(a *app.Context, rToLock string) error {
	c := a.Console()
	if err := locker.IsLocked(rToLock); err != nil {
		return fmt.Errorf("%w", err)
	}

	if !files.Exists(rToLock) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, filepath.Base(rToLock))
	}

	if err := c.ConfirmErr(fmt.Sprintf("Lock %q?", filepath.Base(rToLock)), "n"); err != nil {
		if errors.Is(err, sys.ErrActionAborted) {
			return nil
		}
		return err
	}

	pass, err := passwordConfirm(c)
	if err != nil {
		return err
	}

	if err := locker.Lock(rToLock, pass); err != nil {
		return err
	}

	fmt.Fprintln(a.Writer(), c.SuccessMesg(fmt.Sprintf("database locked: %q", filepath.Base(rToLock))))

	return nil
}

// UnlockRepo unlocks the database.
func UnlockRepo(a *app.Context, rToUnlock string) error {
	if err := locker.IsLocked(rToUnlock); err == nil {
		return fmt.Errorf("%w: %q", locker.ErrFileUnlocked, filepath.Base(rToUnlock))
	}

	rToUnlock = files.EnsureSuffix(rToUnlock, locker.Extension)
	slog.Debug("unlocking database", "name", rToUnlock)

	if !files.Exists(rToUnlock) {
		s := filepath.Base(strings.TrimSuffix(rToUnlock, ".enc"))
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, s)
	}

	c := a.Console()
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
	fmt.Println(c.SuccessMesg("database unlocked"))

	return nil
}

func ProcessBookmarkUpdate(a *app.Context, b *bookmark.Bookmark) error {
	c := a.Console()
	updated, err := updateBookmarkData(a.Context(), c, b)
	if err != nil {
		return err
	}

	if bytes.Equal([]byte(b.Title), []byte(updated.Title)) &&
		bytes.Equal([]byte(b.Desc), []byte(updated.Desc)) {
		return nil
	}

	displayBookmarkChanges(c, b, &updated)

	// Handle user choice
	opt, err := c.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("choose: %w", err)
	}
	switch strings.ToLower(opt) {
	case "y", "yes":
		if err := a.DB.UpdateOne(a.Context(), &updated); err != nil {
			return fmt.Errorf("updating record: %w", err)
		}
		fmt.Print(c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", updated.ID)))
	case "n", "no":
		return nil
	case "e", "edit":
		if err := runEditSession(a,
			[]*bookmark.Bookmark{&updated},
			editor.BookmarkStrategy{},
			editor.WithPostEditionRunE(func(o, u *editor.Record) error {
				return git.UpdateBookmark(a.Cfg, o, u)
			}),
		); err != nil {
			return err
		}
		fmt.Print(c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", updated.ID)))
	}

	return nil
}

// displayBookmarkChanges shows the differences between original and updated bookmarks.
func displayBookmarkChanges(c *ui.Console, b, updated *bookmark.Bookmark) {
	p := c.Palette()
	bid := p.Bold.Sprintf("[%d]", b.ID)
	su := txt.Shorten(updated.URL, 60)
	f := c.Frame()

	f.Reset().Warning(bid + " Found changes in " + p.BrightBlue.Wrap(su, p.Italic) + "\n").Flush()

	if !bytes.Equal([]byte(b.Title), []byte(updated.Title)) {
		f.Reset().Midln(p.BrightCyan.Wrap("Title:", p.Italic)).Flush()
		fmt.Println(txt.DiffColor(txt.Diff([]byte(b.Title), []byte(updated.Title))))
	}

	if !bytes.Equal([]byte(b.Desc), []byte(updated.Desc)) {
		f.Reset().Midln(p.BrightCyan.Wrap("Description:", p.Italic)).Flush()
		fmt.Println(txt.DiffColor(txt.Diff([]byte(b.Desc), []byte(updated.Desc))))
	}
}

// SaveNewBookmark asks the user if they want to save the bookmark.
func SaveNewBookmark(a *app.Context, b *bookmark.Bookmark) error {
	if a.Cfg.Flags.Force {
		return a.DB.InsertMany(a.Context(), []*bookmark.Bookmark{b})
	}

	c := a.Console()
	opt, err := c.Choose("save bookmark?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	switch strings.ToLower(opt) {
	case "n", "no":
		return sys.ErrActionAborted
	case "e", "edit":
		return runEditSession(a, []*bookmark.Bookmark{b}, editor.NewBookmarkStrategy{})
	default:
		if _, err := a.DB.InsertOne(a.Context(), b); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

func updateBookmarkData(ctx context.Context, c *ui.Console, b *bookmark.Bookmark) (bookmark.Bookmark, error) {
	updatedB := *b
	su := txt.Shorten(updatedB.URL, 60)
	p := c.Palette()
	bid := p.Bold.With(p.BgBlue).Sprintf("[%d]", b.ID)

	sc := scraper.New(
		updatedB.URL,
		scraper.WithContext(ctx),
		scraper.WithSpinner(c.Info(bid+" updating bookmark "+p.BrightCyan.Wrap(su, p.Italic)).String()),
	)

	if err := sc.Start(); err != nil {
		return updatedB, err
	}

	updatedB.Title, _ = sc.Title()
	updatedB.Desc, _ = sc.Desc()
	updatedB.FaviconURL, _ = sc.Favicon()
	return updatedB, nil
}

func runEditSession(
	a *app.Context,
	bs []*bookmark.Bookmark,
	es editor.EditStrategy,
	opts ...editor.SessionOption,
) error {
	te, err := editor.NewEditor(a.Cfg.Env.Editor)
	if err != nil {
		return err
	}

	ft := a.Cfg.Name
	if _, ok := es.(editor.JSONStrategy); ok {
		// TODO: add `FileType()` method to Strategy
		ft = "json"
	}

	opts = append(opts,
		editor.WithFileType(ft),
		editor.WithContext(a.Context()),
		editor.WithMeta(editor.NewMeta(a.Cfg.DBName, a.Cfg.Info.Version)),
	)

	session := editor.NewEditSession(a.Console(), a.DB, te, opts...)
	return session.Run(bs, es)
}
