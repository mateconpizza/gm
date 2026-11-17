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
func ScrapeDescriptions(ctx context.Context, sp spinner, bs []*bookmark.Bookmark, log *slog.Logger) error {
	if len(bs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	return scrapeDescriptionsConcurrent(ctx, sp, bs, log)
}

func scrapeDescriptionsConcurrent(ctx context.Context, sp spinner, bs []*bookmark.Bookmark, log *slog.Logger) error {
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

				sc := scraper.New(b.URL, scraper.WithLogger(log))
				if err := sc.Start(ctxTimeout); err != nil {
					log.Error("scraping error", "url", b.URL, "err", err)
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
func EnrichBookmark(ctx context.Context, sp spinner, b *bookmark.Bookmark, log *slog.Logger) *bookmark.Bookmark {
	log = log.With("op", "enrich", "url", b.URL, "id", b.ID)

	if b.Title != "" {
		log.Debug("skipped, title already set")
		return b
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	sc := scraper.New(b.URL, scraper.WithSpinner(sp), scraper.WithLogger(log))
	if err := sc.Start(ctx); err != nil {
		log.Error("scraping error", "err", err)
	}

	if b.Title == "" {
		t, err := sc.Title()
		if err != nil {
			log.Warn("title unavailable", "err", err)
		}
		b.Title = strings.TrimSpace(t)
	}

	if b.Desc == "" {
		d, err := sc.Desc()
		if err != nil {
			log.Warn("description unavailable", "err", err)
		}
		b.Desc = strings.TrimSpace(d)
		if b.Desc == "" {
			log.Debug("no description found")
		}
	}

	if b.Tags == "" || b.Tags == bookmark.DefaultTag {
		src := "keywords"

		tags, err := sc.Keywords()
		if err != nil {
			log.Warn("keywords unavailable", "err", err)
		}

		if tags == "" {
			src = "repo topics"
			repo, err := sc.TagsRepo()
			if err != nil {
				log.Warn("repo topics unavailable", "err", err)
			}
			tags = strings.Join(repo, ",")
		}

		if tags == "" {
			src = "default"
			tags = bookmark.DefaultTag
		}

		log.Debug("tags set", "source", src, "tags", tags)
		b.Tags = tags
	}

	f, err := sc.Favicon()
	if err != nil {
		log.Warn("favicon unavailable", "err", err)
	}
	b.FaviconURL = f

	log.Info("bookmark enriched", "title", b.Title, "has_desc", b.Desc != "")

	return b
}
