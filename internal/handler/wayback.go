package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/deps"
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

func WaybackLatestSnapshot(d *deps.Deps, bs []*bookmark.Bookmark) error {
	var (
		sem   = semaphore.NewWeighted(1)
		wg    sync.WaitGroup
		count atomic.Uint32
		dim   = rotato.FgGray.With(rotato.StyleBold)
	)

	total := uint32(len(bs))

	results := make(chan SnapshotResult, len(bs))

	sp := rotato.New(
		rotato.WithContext(d.Context()),
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

	sp.Start()

	for _, b := range bs {
		if err := sem.Acquire(d.Context(), 1); err != nil {
			sp.Fail(err.Error())
			return fmt.Errorf("acquire semaphore: %w", err)
		}

		wg.Add(1)

		go func(b *bookmark.Bookmark) {
			defer wg.Done()
			defer sem.Release(1)

			count.Add(1)

			sp.UpdateMesg(txt.Shorten(b.URL, 80))

			res := processBookmark(d, b)
			results <- res
		}(b)
	}

	wg.Wait()
	close(results)

	sp.Done()

	return printSummary(d.Console(), results)
}

func processBookmark(d *deps.Deps, b *bookmark.Bookmark) SnapshotResult {
	u := txt.Shorten(b.URL, 60)

	if b.ArchiveURL != "" && b.ArchiveTimestamp != "" && !d.App.Flags.Force {
		return newResult(u, "skipped", wayback.ErrAlreadyArchived.Error())
	}

	ct := wayback.New()
	s, err := ct.ClosestSnapshot(d.Context(), b.URL)
	if err != nil {
		return newResult(u, "error", err.Error())
	}

	b.ArchiveURL = s.ArchiveURL
	b.ArchiveTimestamp = s.ArchiveTimestamp
	if err := d.Repo.UpdateOne(d.Context(), b); err != nil {
		return newResult(u, "error", err.Error())
	}

	return newResult(u, "success", "")
}

func printSummary(c *ui.Console, results <-chan SnapshotResult) error {
	f := c.Frame().Reset()
	var skipped, failed, success []SnapshotResult
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

func waybackMenu[T wayback.SnapshotInfo](c *ui.Console, opts ...menu.Option) *menu.Menu[wayback.SnapshotInfo] {
	opts = append(opts, menu.WithArgs("--color=header:italic:bold:bright-red"),
		menu.WithOutputColor(c.Palette().Enabled()),
		menu.WithHeaderOnly("donate <3 https://archive.org/donate"),
		menu.WithArgs("--cycle"),
	)
	m := menu.New[wayback.SnapshotInfo](opts...)

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
	return txt.PaddedLine(label, absolute+dimmer(relative))
}

func WaybackSnapshots(d *deps.Deps, bs []*bookmark.Bookmark) error {
	c, p := d.Console(), d.Console().Palette()

	ct := wayback.New(wayback.WithByYear(d.App.Flags.Year), wayback.WithLimit(d.App.Flags.Limit))
	for _, b := range bs {
		ctx, cancel := context.WithTimeout(d.Context(), 30*time.Second)
		deadline, _ := ctx.Deadline()

		u := txt.Shorten(b.URL, 60)
		sp := rotato.New(
			rotato.WithPrefix("Snapshots"),
			rotato.WithSpinnerColor(rotato.FgBrightGreen, rotato.StyleBold),
			rotato.WithMessage("Fetching "+p.Italic.Sprint(u)),
			rotato.WithMessageDecorator(func(mesg string) string {
				remaining := max(time.Until(deadline).Round(time.Second), 0)
				return mesg + " " + rotato.DimCountdownDecorator(remaining)
			}),
		)
		sp.Start()

		snapshots, err := ct.Snapshots(ctx, b.URL)
		cancel()

		if err != nil {
			sp.Fail(p.Red.Sprintf("Failed to fetch %s: %v", u, err))
			continue
		}

		f := c.Frame()
		sp.Done(fmt.Sprintf("%d snapshots from %q", len(snapshots), u))
		if b.ArchiveURL != "" {
			f.Midln(formatTime("Current:", b.ArchiveTimestamp)).Flush()
		}

		m := waybackMenu(c, menu.WithFooter(b.URL))
		selected, err := m.Select(snapshots)
		if err != nil {
			if !errors.Is(err, menu.ErrFzfActionAborted) {
				return err
			}

			continue
		}

		snap := selected[0]
		b.ArchiveURL = snap.ArchiveURL
		b.ArchiveTimestamp = snap.ArchiveTimestamp

		updateCtx, updateCancel := context.WithTimeout(d.Context(), 3*time.Second)
		err = d.Repo.UpdateOne(updateCtx, b)
		updateCancel()
		if err != nil {
			return fmt.Errorf("updating: %w", err)
		}

		f.Midln(formatTime("New:", b.ArchiveTimestamp)).Flush()
		fmt.Println(c.SuccessMesg("bookmark updated"))
	}

	return nil
}
