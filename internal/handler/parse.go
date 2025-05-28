package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/haaag/rotato"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/bookmark/scraper"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/terminal"
)

// parseFoundFromBrowser processes the bookmarks found from the import
// browser process.
func parseFoundFromBrowser(
	t *terminal.Term,
	r *repo.SQLiteRepository,
	bs *slice.Slice[bookmark.Bookmark],
) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if err := cleanDuplicates(r, bs); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Row().Ln().Mid("no new bookmark found, skipping import").Ln().Flush()
			return nil
		}
	}

	msg := fmt.Sprintf("scrape missing data from %d bookmarks found?", bs.Len())
	f.Row().Ln().Flush().Clear()
	if !config.App.Force {
		if err := t.ConfirmErr(f.Question(msg).String(), "y"); err != nil {
			if errors.Is(err, terminal.ErrActionAborted) {
				return nil
			}

			return fmt.Errorf("%w", err)
		}
	}

	return scrapeMissingDescription(bs)
}

// cleanDuplicates removes duplicate bookmarks from the import process.
func cleanDuplicates(r *repo.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	originalLen := bs.Len()
	bs.FilterInPlace(func(b *bookmark.Bookmark) bool {
		_, exists := r.Has(b.URL)
		return !exists
	})
	if originalLen != bs.Len() {
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-bs.Len())
		f.Row().Ln().Warning(s).Ln().Flush()
	}

	if bs.Empty() {
		return slice.ErrSliceEmpty
	}

	return nil
}

// scrapeMissingDescription scrapes missing data from bookmarks found from the import
// process.
func scrapeMissingDescription(bs *slice.Slice[bookmark.Bookmark]) error {
	if bs.Len() == 0 {
		return nil
	}
	sp := rotato.New(
		rotato.WithSpinnerColor(rotato.ColorGray),
		rotato.WithMesg("scraping missing data..."),
		rotato.WithMesgColor(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
	)
	sp.Start()
	var wg sync.WaitGroup
	errs := make([]string, 0)
	bs.ForEachMut(func(b *bookmark.Bookmark) {
		wg.Add(1)
		go func(b *bookmark.Bookmark) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			defer wg.Done()
			sc := scraper.New(b.URL, scraper.WithContext(ctx))
			if err := sc.Scrape(); err != nil {
				errs = append(errs, fmt.Sprintf("url %s: %s", b.URL, err.Error()))
				slog.Warn("scraping error", "url", b.URL, "err", err)
			}
			b.Desc = sc.Desc()
		}(b)
	})
	wg.Wait()

	sp.Done("Scraping done")

	return nil
}

// URLValid checks if a string is a valid URL.
func URLValid(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	return parsedURL.Scheme != "" && parsedURL.Host != ""
}
