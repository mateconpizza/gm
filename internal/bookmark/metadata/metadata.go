// Package metadata provides functions to scrape and update various metadata
// for bookmarks (descriptions, title, tags, favicon, etc)
package metadata

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper"
)

var (
	ErrURLEmpty     = errors.New("URL cannot be empty")
	ErrLineNotFound = errors.New("line not found")
)

type spinner interface {
	Start(ctx context.Context)
	Done(mesg ...string)
}

// ScrapeDescriptions scrapes missing data from bookmarks found from the import
// process.
func ScrapeDescriptions(ctx context.Context, sp spinner, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	return scrapeDescriptionsConcurrent(ctx, sp, bs)
}

func scrapeDescriptionsConcurrent(ctx context.Context, sp spinner, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return nil
	}

	sp.Start(ctx)
	defer sp.Done("Scraping done")

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for _, b := range bs {
		g.Go(func() error {
			ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			select {
			case <-ctxTimeout.Done():
				return ctxTimeout.Err()
			default:

				sc := scraper.New(b.URL)
				if err := sc.Start(ctxTimeout); err != nil {
					slog.Warn("scraping error", "url", b.URL, "err", err)
					return nil // just log the error
				}

				b.Desc, _ = sc.Desc()
				return nil
			}
		})
	}

	return g.Wait()
}

// EnrichBookmark updates a Bookmark's title and description by scraping the
// webpage if they are missing.
func EnrichBookmark(ctx context.Context, sp spinner, b *bookmark.Bookmark) *bookmark.Bookmark {
	if b.Title != "" {
		return b
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sc := scraper.New(b.URL, scraper.WithSpinner(sp))
	if err := sc.Start(ctx); err != nil {
		slog.Error("scraping error", "error", err)
	}

	if b.Title == "" {
		t, _ := sc.Title()
		b.Title = strings.TrimSpace(t)
	}

	if b.Desc == "" {
		d, _ := sc.Desc()
		b.Desc = strings.TrimSpace(d)
	}

	if b.Tags == "" || b.Tags == bookmark.DefaultTag {
		tags, _ := sc.Keywords()
		if tags == "" {
			repo, _ := sc.TagsRepo()
			tags = strings.Join(repo, ",")
		}

		if tags == "" {
			tags = bookmark.DefaultTag
		}

		b.Tags = tags
	}

	f, _ := sc.Favicon()
	b.FaviconURL = f

	return b
}
