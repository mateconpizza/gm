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

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper/wayback"
)

var dimmer = func(s string) string { return ansi.BrightBlack.Wrap(" ("+s+")", ansi.Italic) }

type SnapshotResult struct {
	URL   string
	State string // "skipped", "error", "success"
	Msg   string
}

func newResult(u, s, m string) SnapshotResult {
	return SnapshotResult{URL: u, State: s, Msg: m}
}

func WaybackLatestSnapshot(a *app.Context, bs []*bookmark.Bookmark) error {
	// FIX: updateSpinnerWithDeadline
	ctx, cancel := context.WithTimeout(a.Context(), 30*time.Second)
	sem := semaphore.NewWeighted(1)
	var (
		count uint32
		wg    sync.WaitGroup
	)

	c := a.Console()
	f := c.Frame()
	results := make(chan SnapshotResult, len(bs))
	sp := rotato.New(
		rotato.WithPrefix(f.Mid("Fetching snapshots").String()),
		rotato.WithMesgColor(rotato.ColorYellow),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
		rotato.WithFailColorMesg(rotato.ColorBrightRed),
	)
	sp.Start()

	for _, b := range bs {
		if err := sem.Acquire(ctx, 1); err != nil {
			sp.Fail(err.Error())
			cancel()
			return fmt.Errorf("acquire semaphore: %w", err)
		}
		wg.Add(1)

		go func(b *bookmark.Bookmark) {
			defer wg.Done()
			defer sem.Release(1)

			idx := atomic.AddUint32(&count, 1)
			f := frame.New(frame.WithColorBorder(ansi.BrightBlack))
			sp.UpdateMesg(fmt.Sprintf("[%d/%d] %s", idx, len(bs), f.Info(txt.Shorten(b.URL, 80)).String()))

			res := processBookmark(a, b)
			cancel()
			results <- res
		}(b)
	}

	wg.Wait()
	close(results)
	sp.Done()

	cancel()

	return printSummary(c, results)
}

func processBookmark(a *app.Context, b *bookmark.Bookmark) SnapshotResult {
	u := txt.Shorten(b.URL, 60)

	if b.ArchiveURL != "" && b.ArchiveTimestamp != "" && !a.Cfg.Flags.Force {
		return newResult(u, "skipped", wayback.ErrAlreadyArchived.Error())
	}

	ct := wayback.New()
	s, err := ct.ClosestSnapshot(a.Context(), b.URL)
	if err != nil {
		return newResult(u, "error", err.Error())
	}

	b.ArchiveURL = s.ArchiveURL
	b.ArchiveTimestamp = s.ArchiveTimestamp
	if err := a.DB.UpdateOne(a.Context(), b); err != nil {
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

func updateSpinnerWithDeadline(ctx context.Context, sp *rotato.Rotato, prefix string, deadline time.Time) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			remaining := max(time.Until(deadline), 0)
			left := fmt.Sprintf("%.0fs left", remaining.Seconds())
			sp.UpdateMesg(fmt.Sprintf("%s%s", prefix, dimmer(left)))
		}
	}
}

func waybackMenu[T wayback.SnapshotInfo](c *ui.Console, opts ...menu.Option) *menu.Menu[wayback.SnapshotInfo] {
	opts = append(opts, menu.WithArgs("--color=header:italic:bold:bright-red"),
		menu.WithOutputColor(c.Palette().Enabled()),
		menu.WithHeaderOnly("donate <3 https://archive.org/donate"),
		menu.WithArgs("--cycle"),
	)
	m := menu.New[wayback.SnapshotInfo](opts...)

	// format each item `YYYY MMM DD HH:MM (N days ago)`
	m.SetPreprocessor(func(s *wayback.SnapshotInfo) string {
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

func WaybackSnapshots(a *app.Context, bs []*bookmark.Bookmark) error {
	sp := rotato.New(rotato.WithMesg("Fetching wayback machine snapshot"))
	c, p := a.Console(), a.Console().Palette()

	ct := wayback.New(wayback.WithByYear(a.Cfg.Flags.Year), wayback.WithLimit(a.Cfg.Flags.Limit))
	for _, b := range bs {
		sp.Start()

		ctx, cancel := context.WithTimeout(a.Context(), 30*time.Second)
		deadline, _ := ctx.Deadline()

		u := txt.Shorten(b.URL, 60)
		prefix := "Fetching " + p.Italic.Sprint(u) + " snapshots"
		go updateSpinnerWithDeadline(ctx, sp, prefix, deadline)

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
		m.SetItems(snapshots)
		selected, err := m.Select()
		if err != nil {
			if !errors.Is(err, menu.ErrFzfActionAborted) {
				return err
			}

			continue
		}

		snap := selected[0]
		b.ArchiveURL = snap.ArchiveURL
		b.ArchiveTimestamp = snap.ArchiveTimestamp

		updateCtx, updateCancel := context.WithTimeout(a.Context(), 3*time.Second)
		err = a.DB.UpdateOne(updateCtx, b)
		updateCancel()
		if err != nil {
			return fmt.Errorf("updating: %w", err)
		}

		f.Midln(formatTime("New:", b.ArchiveTimestamp)).Flush()
		fmt.Println(c.SuccessMesg("bookmark updated"))
	}

	return nil
}
