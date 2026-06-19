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

	bs, err = confirmRemove(ctx, d, bs)
	if err != nil {
		return err
	}

	return removeRecords(ctx, d, bs)
}
