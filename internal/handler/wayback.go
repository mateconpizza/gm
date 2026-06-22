package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper/wayback"
)

var dimmer = func(s string) string { return ansi.Gray.Wrap(" ("+s+")", ansi.Italic) }

type SnapshotResult struct {
	URL   string
	State string // "skipped", "error", "success"
	Msg   string
}

func newResult(u, s, m string) SnapshotResult {
	return SnapshotResult{URL: u, State: s, Msg: m}
}

func WaybackLatestSnapshot(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	var (
		count atomic.Uint32
		dim   = rotato.FgGray.With(rotato.StyleBold)
		total = uint32(len(bs))
	)

	results := make(chan SnapshotResult, len(bs))

	sp := rotato.New(
		rotato.WithPrefix("Snapshots"),
		rotato.WithPrefixDecorator(func(prefix string) string { // n/N <prefix>
			current := count.Load()
			return fmt.Sprintf("%s %s", prefix, dim.Sprintf("%d/%d", current, total))
		}),
		rotato.WithSpinnerColor(rotato.FgBrightGreen, rotato.StyleBold),
		rotato.WithMessageColor(rotato.FgYellow),
		rotato.WithMessageDecorator(func(mesg string) string { return "Fetching " + mesg }),
		rotato.WithDoneMessageColor(rotato.FgBrightGreen, rotato.StyleItalic),
		rotato.WithFailMessageColor(rotato.FgBrightRed),
	)

	sp.Start(ctx)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for _, b := range bs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				count.Add(1)

				sp.UpdateMesg(txt.Shorten(b.URL, 80))

				res := processBookmark(ctx, d, b)
				results <- res
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		sp.Fail(err.Error())
		close(results)

		return err
	}

	close(results)

	sp.Done()

	return printSummary(d.Console(), results)
}

func processBookmark(ctx context.Context, d *deps.Deps, b *bookmark.Bookmark) SnapshotResult {
	u := txt.Shorten(b.URL, 60)
	app, _ := d.Application(ctx)
	if b.ArchiveURL != "" && b.ArchiveTimestamp != "" && !app.Flags.Force {
		return newResult(u, "skipped", wayback.ErrAlreadyArchived.Error())
	}

	ct := wayback.New(
		wayback.WithTimeout(app.Flags.Duration),
	)

	s, err := ct.ClosestSnapshot(ctx, b.URL)
	if err != nil {
		return newResult(u, "error", err.Error())
	}

	b.ArchiveURL = s.ArchiveURL
	b.ArchiveTimestamp = s.ArchiveTimestamp

	r, _ := d.Repository()
	if err := r.UpdateOne(ctx, b); err != nil {
		return newResult(u, "error", err.Error())
	}

	return newResult(u, "success", "")
}

func printSummary(c *ui.Console, results <-chan SnapshotResult) error {
	var (
		skipped []SnapshotResult
		failed  []SnapshotResult
		success []SnapshotResult
	)

	for r := range results {
		switch r.State {
		case "skipped":
			skipped = append(skipped, r)

		case "error":
			failed = append(failed, r)

		case "success":
			success = append(success, r)
		}
	}

	p := c.Palette()
	f := c.Frame().Reset()
	if len(skipped) > 0 {
		msg := p.BrightYellow.Sprintf("Skipped %d bookmarks", len(skipped))

		f.Warning(msg + dimmer(wayback.ErrAlreadyArchived.Error())).Ln().Flush()

		for _, r := range skipped {
			f.Midln(r.URL).Flush()
		}
	}

	if len(failed) > 0 {
		msg := p.BrightRed.Sprintf("Failed %d bookmarks", len(failed))
		f.Error(msg).Ln().Flush()

		for _, r := range failed {
			f.Midln(r.URL + dimmer(r.Msg)).Flush()
		}
	}

	if len(success) > 0 {
		msg := p.BrightGreen.Sprintf("Updated %d bookmarks", len(success))
		f.Success(msg).Ln().Flush()

		for _, r := range success {
			f.Midln(r.URL).Flush()
		}
	}

	return nil
}

func waybackMenu[T wayback.SnapshotInfo](c *ui.Console, app *application.App, opts ...menu.Option) *menu.Menu[wayback.SnapshotInfo] {
	p := c.Palette()
	opts = append(
		opts,
		menu.WithOutputColor(p.Enabled()),
		menu.WithHeaderOnly(p.BrightRed.Wrap("donate <3 https://archive.org/donate", p.Bold)),
		menu.WithArgs("--cycle"),
	)

	m := picker.New[wayback.SnapshotInfo](app, opts...)

	// format each item `YYYY MMM DD HH:MM (N days ago)`
	m.SetFormatter(func(s *wayback.SnapshotInfo) string {
		absolute, relative := txt.TimeWithAgo(s.ArchiveTimestamp)
		return absolute + dimmer(relative)
	})

	return m
}

// formatTime returns a string formatted YYYY MMM DD HH:MM (N days ago).
func formatTime(label, ts string) string {
	absolute, relative := txt.TimeWithAgo(ts)

	return txt.PaddedLine(
		label,
		absolute+dimmer(relative),
	)
}

// WaybackSnapshots fetches and updates archive snapshots for each bookmark.
func WaybackSnapshots(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	ct := wayback.New(
		wayback.WithByYear(app.Flags.Year),
		wayback.WithLimit(app.Flags.Limit),
		wayback.WithTimeout(app.Flags.Duration),
	)

	c := d.Console()

	for _, b := range bs {
		snapshots, err := fetchSnapshots(ctx, c, ct, b)
		if err != nil {
			slog.Debug("wayback snapshot:", "error", err)
			continue
		}

		snap, err := selectSnapshot(ctx, d, b, snapshots)
		if err != nil {
			if errors.Is(err, menu.ErrFzfActionAborted) {
				continue
			}

			return err
		}

		if err := applySnapshot(ctx, d, b, snap); err != nil {
			return err
		}
	}

	return nil
}

// fetchSnapshots fetches the wayback snapshots for a single bookmark.
func fetchSnapshots(
	ctx context.Context,
	c *ui.Console,
	ct *wayback.WaybackMachine,
	b *bookmark.Bookmark,
) ([]wayback.SnapshotInfo, error) {
	p := c.Palette()
	u := txt.Shorten(b.URL, 60)

	timeout := ct.Timeout()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	deadline, _ := ctx.Deadline()

	sp := rotato.New(
		rotato.WithPrefix("Snapshots"),
		rotato.WithSpinnerColor(
			rotato.FgBrightGreen,
			rotato.StyleBold,
		),
		rotato.WithMessage("fetching "+p.Italic.Sprint(u)),
		rotato.WithMessageDecorator(func(mesg string) string {
			remaining := max(
				time.Until(deadline).Round(time.Second),
				0,
			)

			return mesg + " " + rotato.DimCountdownDecorator(remaining)
		}),
	)

	sp.Start(ctx)

	snapshots, err := ct.Snapshots(ctx, b.URL)
	if err != nil {
		sp.Fail(
			p.Red.Sprintf(
				"Failed to fetch %s: %v",
				u,
				err,
			),
		)

		return nil, err
	}

	sp.Done(fmt.Sprintf(
		"%d snapshots from %s",
		len(snapshots),
		p.Dim.Wrap(u, p.Italic),
	))

	return snapshots, nil
}

// selectSnapshot shows the current archive info.
func selectSnapshot(
	ctx context.Context,
	d *deps.Deps,
	b *bookmark.Bookmark,
	snaps []wayback.SnapshotInfo,
) (wayback.SnapshotInfo, error) {
	c := d.Console()

	if b.ArchiveURL != "" {
		c.Frame().
			Midln(formatTime("Current:", b.ArchiveTimestamp)).
			Flush()
	}

	app, err := d.Application(ctx)
	if err != nil {
		return wayback.SnapshotInfo{}, err
	}

	m := waybackMenu(
		c,
		app,
		menu.WithFooter(b.URL),
	)

	selected, err := m.Select(snaps)
	if err != nil {
		return wayback.SnapshotInfo{}, err
	}

	return selected[0], nil
}

// applySnapshot persists the selected snapshot to the bookmark and reports the result.
func applySnapshot(
	ctx context.Context,
	d *deps.Deps,
	b *bookmark.Bookmark,
	snap wayback.SnapshotInfo,
) error {
	b.ArchiveURL = snap.ArchiveURL
	b.ArchiveTimestamp = snap.ArchiveTimestamp

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	r, err := d.Repository()
	if err != nil {
		return err
	}

	if err := r.UpdateOne(ctx, b); err != nil {
		return fmt.Errorf("updating: %w", err)
	}

	c := d.Console()

	c.Frame().
		Midln(formatTime("New:", b.ArchiveTimestamp)).
		Flush()

	return c.Print(
		ctx,
		c.SuccessMesg("bookmark updated\n"),
	)
}
