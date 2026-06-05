// Package handler handles parsing and processing of bookmark data operations.
package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/scraper"
)

// QR handles creation, rendering or opening of QR-Codes.
func QR(_ context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	qrFn := func(b *bookmark.Bookmark) error {
		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		p := d.Console().Palette()
		var sb strings.Builder
		sb.WriteString(p.Bold.Sprint(b.Title + "\n"))
		sb.WriteString(p.Italic.Sprint(b.URL + "\n"))
		sb.WriteString(qrcode.String())
		fmt.Fprint(d.Writer(), sb.String())

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

// Open opens the URLs in the browser for the bookmarks in the provided slice.
func Open(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	const maxGoroutines = 5
	n := len(bs)
	p := d.Console().Palette()
	s := fmt.Sprintf("%s %d bookmarks", p.BrightGreen.Wrap("open", p.Bold), n)

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	if err := d.Console().ConfirmLimit(ctx, n, maxGoroutines, s, app.Flags.Force); err != nil {
		return err
	}

	if err := openInBrowser(ctx, bs); err != nil {
		return err
	}

	return recordVisits(ctx, d, bs)
}

// openInBrowser concurrently opens each bookmark URL in the browser.
func openInBrowser(ctx context.Context, bs []*bookmark.Bookmark) error {
	sp := rotato.New(
		rotato.WithMessage("opening bookmarks..."),
		rotato.WithMessageColor(rotato.FgBrightGreen),
		rotato.WithSpinnerColor(rotato.FgBrightGreen),
	)

	sp.Start(ctx)
	defer sp.Done()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for _, b := range bs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err := sys.OpenInBrowser(ctx, b.URL); err != nil {
					return fmt.Errorf("open error: %w", err)
				}

				return nil
			}
		})
	}

	return g.Wait()
}

// recordVisits increments the visit counter for each bookmark.
func recordVisits(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}

	for _, b := range bs {
		if err := r.AddVisit(ctx, b.ID); err != nil {
			return err
		}
	}

	return nil
}

// Edit returns a BookmarkAction configured with a specific strategy.
func Edit(ctx context.Context, es editor.EditStrategy) func(context.Context, *deps.Deps, []*bookmark.Bookmark) error {
	return func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
		const maxItems = 10

		app, err := d.Application(ctx)
		if err != nil {
			return err
		}

		p := d.Console().Palette()
		q := fmt.Sprintf("%s %d bookmarks", p.BrightGreen.Wrap("edit", p.Bold), len(bs))

		if err := d.Console().ConfirmLimit(ctx, len(bs), maxItems, q, app.Flags.Force); err != nil {
			return err
		}

		opt := editor.WithPostEditionRunE(func(old, fresh *editor.Record) error {
			return gitops.Update(ctx, app, old, fresh)
		})

		return runEditSession(ctx, d, bs, es, opt)
	}
}

// LockRepo locks the database.
func LockRepo(ctx context.Context, d *deps.Deps, rToLock string) error {
	c := d.Console()
	if err := locker.IsLocked(rToLock); err != nil {
		return fmt.Errorf("%w", err)
	}

	if !files.Exists(rToLock) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, filepath.Base(rToLock))
	}

	if err := c.ConfirmErr(ctx, fmt.Sprintf("Lock %q?", filepath.Base(rToLock)), "n"); err != nil {
		if errors.Is(err, sys.ErrActionAborted) {
			return nil
		}
		return err
	}

	pass, err := passwordConfirm(ctx, c)
	if err != nil {
		return err
	}

	if err := locker.Lock(rToLock, pass); err != nil {
		return err
	}

	fmt.Fprintln(d.Writer(), c.SuccessMesg(fmt.Sprintf("database locked: %q", filepath.Base(rToLock))))

	return nil
}

// UnlockRepo unlocks the database.
func UnlockRepo(ctx context.Context, d *deps.Deps, rToUnlock string) error {
	if err := locker.IsLocked(rToUnlock); err == nil {
		return fmt.Errorf("%w: %q", locker.ErrFileUnlocked, filepath.Base(rToUnlock))
	}

	rToUnlock = files.EnsureSuffix(rToUnlock, locker.Extension)
	slog.Debug("unlocking database", "name", rToUnlock)

	if !files.Exists(rToUnlock) {
		s := filepath.Base(strings.TrimSuffix(rToUnlock, ".enc"))
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, s)
	}

	c := d.Console()
	if err := c.ConfirmErr(ctx, fmt.Sprintf("Unlock %q?", filepath.Base(rToUnlock)), "y"); err != nil {
		return fmt.Errorf("%w", err)
	}

	s, err := c.InputPassword(ctx, "Password: ")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := locker.Unlock(rToUnlock, s); err != nil {
		fmt.Fprintln(d.Writer())
		return fmt.Errorf("%w", err)
	}

	fmt.Fprintln(d.Writer())
	fmt.Fprintln(d.Writer(), c.SuccessMesg("database unlocked"))

	return nil
}

func MigrationsStatus(ctx context.Context, d *deps.Deps) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}

	c := d.Console()
	p, f := c.Palette(), c.Frame()
	header := func() string {
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}
	f.CustomFunc(header, p.Bold.Sprint("Configuring database")).Ln()

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if err = db.UpdateAppVersion(ctx, r, app.Version()); err != nil {
		return fmt.Errorf("app version update failed: %w", err)
	}

	schemaVer, err := db.CurrentSchemaVersion(ctx, r)
	if err != nil {
		return err
	}

	sqlVer, err := db.SQLiteVersion(ctx, r)
	if err != nil {
		return err
	}

	const padding = 28
	f.Success(txt.PaddedLineWithPad("schema version", p.BrightGreen.Sprint(schemaVer)+"\n", padding)).
		Success(txt.PaddedLineWithPad("sqlite version", p.BrightMagenta.Sprint(sqlVer)+"\n", padding)).
		Rowln().
		Flush()

	return nil
}

func ProcessBookmarkUpdate(ctx context.Context, d *deps.Deps, b *bookmark.Bookmark) error {
	c := d.Console()
	updated, err := updateBookmarkData(ctx, c, b)
	if err != nil {
		return err
	}

	if bytes.Equal([]byte(b.Title), []byte(updated.Title)) &&
		bytes.Equal([]byte(b.Desc), []byte(updated.Desc)) {
		return nil
	}

	displayBookmarkChanges(d.Writer(), c, b, &updated)

	r, err := d.Repository()
	if err != nil {
		return err
	}

	// Handle user choice
	opt, err := c.Choose(ctx, "save changes?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("choose: %w", err)
	}
	switch strings.ToLower(opt) {
	case "y", "yes":
		if err := r.UpdateOne(ctx, &updated); err != nil {
			return fmt.Errorf("updating record: %w", err)
		}
		fmt.Fprint(d.Writer(), c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", updated.ID)))
	case "n", "no":
		return nil
	case "e", "edit":
		if err := runEditSession(
			ctx,
			d,
			[]*bookmark.Bookmark{&updated},
			editor.BookmarkStrategy{},
			editor.WithPostEditionRunE(func(o, u *editor.Record) error {
				app, err := d.Application(ctx)
				if err != nil {
					return err
				}
				return gitops.Update(ctx, app, o, u)
			}),
		); err != nil {
			return err
		}
		fmt.Fprint(d.Writer(), c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", updated.ID)))
	}

	return nil
}

// displayBookmarkChanges shows the differences between original and updated bookmarks.
func displayBookmarkChanges(w io.Writer, c *ui.Console, b, updated *bookmark.Bookmark) {
	p := c.Palette()
	bid := p.Bold.Sprintf("[%d]", b.ID)
	su := txt.Shorten(updated.URL, 60)
	f := c.Frame()

	f.Reset().Warning(bid + " Found changes in " + p.BrightBlue.Wrap(su, p.Italic) + "\n").Flush()

	if !bytes.Equal([]byte(b.Title), []byte(updated.Title)) {
		f.Reset().Midln(p.BrightCyan.Wrap("Title:", p.Italic)).Flush()
		fmt.Fprintln(w, txt.DiffColor(txt.Diff([]byte(b.Title), []byte(updated.Title))))
	}

	if !bytes.Equal([]byte(b.Desc), []byte(updated.Desc)) {
		f.Reset().Midln(p.BrightCyan.Wrap("Description:", p.Italic)).Flush()
		fmt.Fprintln(w, txt.DiffColor(txt.Diff([]byte(b.Desc), []byte(updated.Desc))))
	}
}

func updateBookmarkData(ctx context.Context, c *ui.Console, b *bookmark.Bookmark) (bookmark.Bookmark, error) {
	updatedB := *b
	su := txt.Shorten(updatedB.URL, 60)
	p := c.Palette()
	bid := p.Bold.With(p.BgBlue).Sprintf("[%d]", b.ID)

	sc := scraper.New(
		updatedB.URL,
		scraper.WithSpinner(c.Info(bid+" updating bookmark "+p.BrightCyan.Wrap(su, p.Italic)).String()),
	)

	if err := sc.Start(ctx); err != nil {
		return updatedB, err
	}

	updatedB.Title, _ = sc.Title()
	updatedB.Desc, _ = sc.Desc()
	updatedB.FaviconURL, _ = sc.Favicon()
	return updatedB, nil
}

func runEditSession(
	ctx context.Context,
	d *deps.Deps,
	bs []*bookmark.Bookmark,
	es editor.EditStrategy,
	opts ...editor.SessionOption,
) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	te, err := editor.NewEditor(app.Env.Editor)
	if err != nil {
		return err
	}

	ft := app.Name
	if _, ok := es.(editor.JSONStrategy); ok {
		// TODO: add `FileType()` method to Strategy
		ft = "json"
	}

	opts = append(
		opts,
		editor.WithFileType(ft),
		editor.WithMeta(editor.NewMeta(app.DBName, app.Version())),
	)

	r, err := d.Repository()
	if err != nil {
		return err
	}

	session := editor.NewEditSession(d.Console(), r, te, opts...)
	return session.Run(ctx, bs, es)
}
