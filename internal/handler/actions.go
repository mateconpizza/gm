// Package handler handles parsing and processing of bookmark data operations.
package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/bookmark/status"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
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

		opt := editor.WithPostEditionRunE(func(old, fresh *bookmark.Bookmark) error {
			return gitops.Update(ctx, app, old, fresh)
		})

		return runEditSession(ctx, d, bs, es, opt)
	}
}

func Yank(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	c := d.Console()
	p := c.Palette()

	msg := fmt.Sprintf(
		"%s %d bookmarks to system clipboard",
		p.BrightGreen.Wrap("copy", p.Bold),
		len(bs),
	)

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if err := c.ConfirmLimit(ctx, len(bs), 10, msg, app.Flags.Force); err != nil {
		return err
	}

	content, err := clipboardContent(bs, app.Flags.JSON)
	if err != nil {
		return err
	}

	if !app.Flags.Force && !app.Flags.Yes {
		c.ClearLine(2)
	}

	if err := sys.CopyClipboard(content); err != nil {
		return err
	}

	return c.Print(
		ctx,
		c.SuccessMesg("copied ", len(bs), " bookmarks to system clipboard\n"),
	)
}

// HTTPStatusCheck refreshes and persists bookmark HTTP status information.
func HTTPStatusCheck(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return bookmark.ErrBookmarkNotFound
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	c, p := d.Console(), d.Console().Palette()
	q := fmt.Sprintf("checking %s of %d bookmarks", p.BrightGreen.Wrap("status", p.Bold), len(bs))
	if err = c.ConfirmLimit(ctx, len(bs), 15, q, app.Flags.Force); err != nil {
		return sys.ErrActionAborted
	}

	bs, err = status.Check(ctx, c, bs)
	if err != nil {
		return err
	}

	if err := saveStatusUpdates(ctx, d, bs); err != nil {
		return err
	}

	return c.Print(
		ctx,
		d.Console().SuccessMesg("bookmark status checked\n"),
	)
}

// HTTPStatus prints bookmark HTTP status information.
func HTTPStatus(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return nil
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if app.Flags.Field != "" {
		return printer.Display(ctx, d.Console(), formatter.HTTPStatusCode.String(), bs)
	}

	headers := []string{"Code", "Count", "Description"}
	rows := [][]string{}
	footer := []string{}

	type Row struct {
		description string
		count       int
	}

	newItems := make(map[int]Row, len(bs))
	c := d.Console()
	p := c.Palette()

	for i := range bs {
		b := bs[i]
		statusText := b.HTTPStatusText
		if statusText == "" {
			statusText = "Unassigned"
		}

		r := newItems[b.HTTPStatusCode]
		r.description = txt.HTTPStatusCodeColor(b.HTTPStatusCode, p).
			Sprint(statusText)
		r.count++
		newItems[b.HTTPStatusCode] = r
	}

	codes := make([]int, 0, len(newItems))
	for code := range newItems {
		codes = append(codes, code)
	}

	sort.Ints(codes)

	for _, code := range codes {
		v := newItems[code]

		rows = append(rows, []string{
			strconv.Itoa(code),
			strconv.Itoa(v.count),
			v.description,
		})
	}

	return c.Print(ctx, txt.CreateSimpleTable(headers, rows, footer...))
}

// UpdateMetadata refreshes metadata for the provided bookmarks.
func UpdateMetadata(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	c, p := d.Console(), d.Console().Palette()

	s := fmt.Sprintf("update metadata of %d bookmarks", len(bs))
	if err := c.ConfirmLimit(ctx, len(bs), 10, s, app.Flags.Force); err != nil {
		return sys.ErrActionAborted
	}

	if len(bs) > 1 {
		c.Frame().Reset().
			Headerln(p.Yellow.Sprintf("Updating %d bookmarks", len(bs))).
			Rowln().
			Flush()
	}

	for _, b := range bs {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := processMetadataUpdate(ctx, d, b); err != nil {
			return err
		}
	}

	return nil
}

func clipboardContent(bs []*bookmark.Bookmark, asJSON bool) (string, error) {
	if asJSON {
		b, err := port.ToJSON(bs)
		if err != nil {
			return "", err
		}

		return string(b), nil
	}

	var sb strings.Builder
	for _, b := range bs {
		sb.WriteString(b.URL)
		sb.WriteByte('\n')
	}

	return sb.String(), nil
}

// processMetadataUpdate updates a bookmark's metadata after user confirmation.
func processMetadataUpdate(ctx context.Context, d *deps.Deps, b *bookmark.Bookmark) error {
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

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	// Handle user choice
	opt, err := c.Choose(ctx, "save changes?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("choose: %w", err)
	}

	switch strings.ToLower(opt) {
	case "n", "no":
		return nil

	case "y", "yes":
		if err := r.UpdateOne(ctx, &updated); err != nil {
			return fmt.Errorf("updating record: %w", err)
		}
		if err := gitops.Update(ctx, app, b, &updated); err != nil {
			return err
		}
		fmt.Fprint(d.Writer(), c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", updated.ID)))

	case "e", "edit":
		if err := runEditSession(
			ctx,
			d,
			[]*bookmark.Bookmark{&updated},
			editor.BookmarkStrategy{},
			editor.WithPostEditionRunE(func(o, u *bookmark.Bookmark) error {
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
		fmt.Fprintln(w, txt.DiffColorize(txt.Diff([]byte(b.Title), []byte(updated.Title))))
	}

	if !bytes.Equal([]byte(b.Desc), []byte(updated.Desc)) {
		f.Reset().Midln(p.BrightCyan.Wrap("Description:", p.Italic)).Flush()
		fmt.Fprintln(w, txt.DiffColorize(txt.Diff([]byte(b.Desc), []byte(updated.Desc))))
	}
}

func updateBookmarkData(ctx context.Context, c *ui.Console, b *bookmark.Bookmark) (bookmark.Bookmark, error) {
	updatedB := *b
	su := txt.Shorten(updatedB.URL, 60)
	p := c.Palette()
	bid := p.Bold.With(p.Blue).Sprintf("[%d]", b.ID)

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

	opts = append(
		opts,
		editor.WithMeta(editor.NewMeta(app.DBName, app.Version())),
	)

	r, err := d.Repository()
	if err != nil {
		return err
	}

	session := editor.NewEditSession(d.Console(), r, te, opts...)
	return session.Run(ctx, bs, es)
}

func saveStatusUpdates(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	m, err := gitops.NewManager(app)
	if err != nil {
		return err
	}

	gr := gitops.NewRepo(
		m,
		r.BaseName(),
		gitops.RepoStatsReader(r),
	)

	for _, b := range bs {
		if b.HTTPStatusCode == http.StatusTooManyRequests {
			continue
		}

		if err := r.UpdateOne(ctx, b); err != nil {
			return err
		}

		if app.GitEnabled() {
			if err := m.Update(ctx, gr, b, b, files.RemoveEmptyDirs); err != nil {
				return err
			}
		}
	}

	if app.GitEnabled() {
		err := m.SaveChanges(
			ctx,
			gr,
			fmt.Sprintf("[%s] http status updated", gr.Name()),
		)

		if err != nil && !errors.Is(err, git.ErrGitUpToDate) {
			return err
		}
	}

	return nil
}
