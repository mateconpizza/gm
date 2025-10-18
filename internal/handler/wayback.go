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
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper/wayback"
)

var dimmer = func(s string) string { return color.BrightGray(" (" + s + ")").Italic().String() }

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
	ctx, cancel := context.WithTimeout(a.Ctx, 30*time.Second)
	sem := semaphore.NewWeighted(1)
	var (
		count uint32
		wg    sync.WaitGroup
	)

	results := make(chan SnapshotResult, len(bs))
	sp := rotato.New(
		rotato.WithPrefix(a.Console.Frame.Mid("Fetching snapshots").String()),
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
			f := frame.New(frame.WithColorBorder(color.Gray))
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

	return printSummary(a.Console, results)
}

func processBookmark(a *app.Context, b *bookmark.Bookmark) SnapshotResult {
	u := txt.Shorten(b.URL, 60)

	if b.ArchiveURL != "" && b.ArchiveTimestamp != "" && !a.Cfg.Flags.Force {
		return newResult(u, "skipped", wayback.ErrAlreadyArchived.Error())
	}

	ct := wayback.New()
	s, err := ct.ClosestSnapshot(a.Ctx, b.URL)
	if err != nil {
		return newResult(u, "error", err.Error())
	}

	b.ArchiveURL = s.ArchiveURL
	b.ArchiveTimestamp = s.ArchiveTimestamp
	if err := a.DB.UpdateOne(a.Ctx, b); err != nil {
		return newResult(u, "error", err.Error())
	}

	return newResult(u, "success", "")
}

func printSummary(c *ui.Console, results <-chan SnapshotResult) error {
	c.Frame.Reset()
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

	if len(skipped) > 0 {
		msg := color.BrightYellow("Skipped", len(skipped), "bookmarks").String()
		c.Frame.Warning(msg + dimmer(wayback.ErrAlreadyArchived.Error())).Ln().Flush()
		for _, r := range skipped {
			c.Frame.Midln(r.URL).Flush()
		}
	}

	if len(failed) > 0 {
		msg := color.BrightRed("Failed", len(failed), "bookmarks").String()
		c.Frame.Error(msg).Ln().Flush()
		for _, r := range failed {
			c.Frame.Midln(r.URL + dimmer(r.Msg)).Flush()
		}
	}

	if len(success) > 0 {
		msg := color.BrightGreen("Updated", len(success), "bookmarks").String()
		c.Frame.Success(msg).Ln().Flush()
		for _, r := range success {
			c.Frame.Midln(r.URL).Flush()
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

func waybackMenu[T wayback.SnapshotInfo]() *menu.Menu[wayback.SnapshotInfo] {
	donate := color.BrightRed("donate <3 ").Bold().String()
	u := "https://archive.org/donate"

	m := menu.New[wayback.SnapshotInfo](
		menu.WithHeader(donate+u, false),
		menu.WithArgs("--cycle"),
	)

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
	cfg := config.New()
	sp := rotato.New(rotato.WithMesg("Fetching wayback machine snapshot"))
	m := waybackMenu()

	ct := wayback.New(wayback.WithByYear(cfg.Flags.Year), wayback.WithLimit(cfg.Flags.Limit))
	for _, b := range bs {
		sp.Start()

		ctx, cancel := context.WithTimeout(a.Ctx, 30*time.Second)
		deadline, _ := ctx.Deadline()

		u := txt.Shorten(b.URL, 60)
		prefix := "Fetching " + color.Text(u).Italic().String() + " snapshots"
		go updateSpinnerWithDeadline(ctx, sp, prefix, deadline)
		snapshots, err := ct.Snapshots(ctx, b.URL)
		cancel()
		if err != nil {
			sp.Done()
			return err
		}

		sp.Done(fmt.Sprintf("%d snapshots from %q", len(snapshots), u))
		if b.ArchiveURL != "" {
			a.Console.Frame.Midln(formatTime("Current:", b.ArchiveTimestamp)).Flush()
		}

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

		if cfg.Flags.Open {
			a.Console.Frame.Midln(formatTime("Open:", b.ArchiveTimestamp)).Flush()
			return sys.OpenInBrowser(snap.ArchiveURL)
		}

		ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
		err = a.DB.UpdateOne(ctx, b)
		cancel()
		if err != nil {
			return err
		}

		a.Console.Frame.Midln(formatTime("New:", b.ArchiveTimestamp)).Flush()
		fmt.Print(a.Console.SuccessMesg("bookmark updated\n"))
	}

	return nil
}
