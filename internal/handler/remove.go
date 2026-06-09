package handler

import (
	"context"

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// Remove prompts the user the records to remove.
func Remove(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	if err := validateRemove(bs, app.Flags.Force); err != nil {
		return err
	}

	if app.Flags.Force || app.Flags.Yes {
		return removeRecords(ctx, d, bs)
	}

	c := d.Console()
	p := c.Palette()

	title := p.BrightRed.With(p.Bold).
		Sprint("Remove Bookmarks")
	subtitle := p.Dim.With(p.Italic).
		Sprint("this action cannot be undone")
	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")
	header := func() string { return p.BrightRed.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold) }

	c.Frame().
		CustomFunc(header, title+comment).Ln().
		Headerln(subtitle).
		Rowln().Flush()

	t := d.Console().Term()
	defer t.CancelInterruptHandler()

	items := make([]bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		items = append(items, *bs[i])
	}

	items, err = confirmRemove(ctx, d, items)
	if err != nil {
		return err
	}

	toRemove := make([]*bookmark.Bookmark, 0, len(items))
	for i := range items {
		toRemove = append(toRemove, &items[i])
	}

	return removeRecords(ctx, d, toRemove)
}
