package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// RemoveRepo removes a repo.
func RemoveRepo(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	if !files.Exists(app.Path.DB()) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, app.Path.DB())
	}

	c, p := d.Console(), d.Console().Palette()
	if filepath.Base(app.Path.DB()) == application.MainDBName && !app.Flags.Force {
		f := p.BrightYellow.With(ansi.Italic).Sprint("--force")
		return fmt.Errorf("%w: removing the main database requires %s", ErrInvalidOption, f)
	}

	if !app.Flags.Force && !app.Flags.Yes {
		title := p.BrightRed.With(p.Bold).Sprint("Remove Database/s")
		subtitle := p.Dim.With(p.Italic).Sprint("this action cannot be undone")
		header := func() string {
			return p.BrightRed.Wrap(txt.GlyphBlackSquare.Prefix(" "), p.Bold)
		}

		c.Frame().
			CustomFunc(header, title).Ln().
			Headerln(subtitle).
			Rowln().
			Flush()

		fmt.Fprint(d.Writer(), summary.RepoFromPath(ctx, d, app.Path.DB(), app.Path.Backup()))
		err := c.ConfirmErr(ctx, p.BrightRed.Wrap("remove", p.Bold)+" "+filepath.Base(app.Path.DB())+"?", "n")
		if err != nil {
			return err
		}
	}

	if err := RemoveBackups(ctx, d); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return err
		}
	}

	if err := files.Remove(app.Path.DB()); err != nil {
		return err
	}

	dbName := files.StripSuffixes(filepath.Base(app.Path.DB()))
	fmt.Fprintln(d.Writer(), c.SuccessMesg("database "+dbName+" removed"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	fn := app.Path.DB()
	dbName := files.StripSuffixes(filepath.Base(fn))
	fs, err := files.List(app.Path.Backup(), "*_"+dbName+".db*") // match YYYYMMDD-HHMMSS_dbname.db
	if err != nil {
		return err
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}
	if app.Flags.Yes || app.Flags.Force {
		return removeSlicePath(ctx, d, fs)
	}

	filesToRemove := make([]string, 0, len(fs))
	c, p := d.Console(), d.Console().Palette()
actionLoop:
	for {
		opt, err := c.Choose(ctx, p.BrightRed.Wrap("remove", p.Bold)+" backups?", []string{"all", "no", "select"}, "n")
		if err != nil {
			return err
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			c.ReplaceLine(c.Warning(p.BrightYellow.Sprint("skipping") + " backup/s").StringReset())
			break actionLoop
		case "a", "all":
			filesToRemove = append(filesToRemove, fs...)
			break actionLoop
		case "s", "select":
			c.SetReader(os.Stdin)
			c.SetWriter(os.Stdout)
			selected, err := selection(
				fs,
				func(p *string) string {
					return summary.BackupWithFmtDateFromPath(ctx, d.Console(), *p)
				},
				menu.WithArgs("--cycle"),
				menu.WithConfig(app.Menu),
				menu.WithMultiSelection(),
				menu.WithOutputColor(app.Flags.Color),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(fn))),
				menu.WithPreview(app.Cmd+" db --db=./backup/{1} info"),
			)
			if err != nil {
				return err
			}
			c.ClearLine(1)
			filesToRemove = append(filesToRemove, selected...)
			break actionLoop
		}
	}

	header := func() string {
		return p.BrightRed.Wrap(txt.GlyphBlackSquare.Prefix(" "), p.Bold)
	}
	c.Frame().CustomFunc(header, p.BrightRed.Sprint("Remove")+" backups\n").Rowln().Flush()

	return removeSlicePath(ctx, d, filesToRemove)
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(ctx context.Context, d *deps.Deps, dbs []string) error {
	c := d.Console()
	f := c.Frame()
	n := len(dbs)
	if n == 0 {
		return ErrNoItems
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	if n > 1 && !app.Flags.Yes {
		for i := range n {
			f.Midln(summary.RepoRecordsFromPath(ctx, d.Console(), dbs[i]))
		}
		f.Flush()

		msg := fmt.Sprintf("%s %d item/s", c.Palette().BrightRed.Sprint("removing"), n)
		if err := c.ConfirmErr(ctx, msg+", continue?", "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp := rotato.New(
		rotato.WithMessage("removing database..."),
		rotato.WithMessageColor(rotato.FgYellow),
	)
	sp.Start(ctx)

	rmRepo := func(p string) error {
		if err := files.Remove(p); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	}

	for i := range n {
		if err := rmRepo(dbs[i]); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp.Done()

	fmt.Fprintln(d.Writer(), c.SuccessMesg(fmt.Sprintf("%d item/s removed", n)))

	return nil
}

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

// DropDatabase drops a database.
func DropDatabase(ctx context.Context, d *deps.Deps) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	c := d.Console()
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	if app.Flags.Yes || app.Flags.Force {
		fmt.Fprintln(d.Writer(), c.SuccessMesg("database dropped"))

		return r.DropSecure(ctx)
	}

	f, p := c.Frame(), c.Palette()
	title := p.BrightRed.
		Wrap("Drop All Records", p.Bold)
	subtitle := p.Dim.With(p.Italic).
		Sprint("this action cannot be undone")
	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")
	header := func() string {
		return p.BrightRed.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	s, err := summary.Info(ctx, d)
	if err != nil {
		return err
	}

	f.CustomFunc(header, title+comment).Ln().
		Headerln(subtitle).
		Rowln().
		Text(s).
		Rowln().Flush()

	q := "continue?"
	if r.Name() == application.MainDBName {
		q = c.WarningMesg("dropping \"main\" database, continue?")
	}

	if err := c.ConfirmErr(ctx, q, "n"); err != nil {
		return err
	}

	if err := r.DropSecure(ctx); err != nil {
		return err
	}

	if err := c.Term().Print(ctx, c.SuccessMesg("database dropped\n")); err != nil {
		return err
	}

	return gitops.Drop(ctx, app, c)
}
