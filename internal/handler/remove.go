package handler

import (
	"context"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/deps"
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

	if err := validateRemove(bs, app.Flags.Force, app.Abort()); err != nil {
		return err
	}

	if app.Flags.Force || app.Flags.Yes {
		return removeRecords(ctx, d, bs)
	}

	p := d.Console().Palette()
	value := func(s string) string {
		return p.BrightYellow.With(p.Italic).Sprint(
			files.CollapseHomeDir(s),
		)
	}

	d.Console().NewBannerBuilder().
		WithTitle("Remove Bookmarks").
		WithTitleColor(p.BrightRed.With(p.Bold).Sprint).
		WithTitleGlyph(app.Glyphs().Del).
		WithSubtitle("this action cannot be undone").
		WithComment(" (ctrl-c to exit)").
		Build().
		Rowln().
		Info("source: " + value(app.Path.DB()+"\n")).
		Rowln().
		Flush()

	t := d.Console().Term()
	defer t.CancelInterruptHandler()

	bs, err = confirmRemove(ctx, d, bs)
	if err != nil {
		return err
	}

	return removeRecords(ctx, d, bs)
}
