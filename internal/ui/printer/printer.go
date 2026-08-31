// Package printer provides functions to format and print bookmark data,
// including records, tags, and repository information.
package printer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	menu "github.com/mateconpizza/go-fzf"
	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var (
	ErrInvalidFormat = errors.New("invalid format")
	ErrUnknownFormat = errors.New("unknown format")
)

func MenuPreview(c *ui.Console, bs []*bookmark.Bookmark, f string) error {
	fm, err := formatter.New(formatter.Format(f))
	if err != nil {
		return err
	}

	for i := range bs {
		fmt.Fprint(c.Writer(), fm.Render(c, bs[i]))
	}

	return nil
}

// Records prints the bookmarks in a frame format with the given colorscheme.
func Records(ctx context.Context, c *ui.Console, bs []*bookmark.Bookmark) error {
	var buf strings.Builder
	lastIdx := len(bs) - 1
	for i, b := range bs {
		buf.WriteString(formatter.FrameFunc(c, b))
		if i != lastIdx {
			buf.WriteByte('\n')
		}
	}

	return c.Print(ctx, buf.String())
}

// TagsList lists the tags.
func TagsList(ctx context.Context, w io.Writer, p string) error {
	r, err := db.New(ctx, p)
	if err != nil {
		return err
	}
	defer r.Close()

	tags, err := db.TagsList(ctx, r)
	if err != nil {
		return fmt.Errorf("tagslist: %w", err)
	}

	fmt.Fprintln(w, strings.Join(tags, "\n"))

	return nil
}

// Print formats the bookmarks with the given fn.
func Print(ctx context.Context, c *ui.Console, bs []*bookmark.Bookmark, fn formatter.Func) error {
	var buf strings.Builder
	for i := range bs {
		line := fn(c, bs[i])
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	return c.Print(ctx, buf.String())
}

// Notes formats the bookmarks notes.
func Notes(ctx context.Context, c *ui.Console, bs []*bookmark.Bookmark) error {
	f := frame.New(
		frame.WithWriter(c.Writer()),
		frame.WithBorders(frame.NewBorders("# ", "", "## ", "")),
	)

	w := c.MinWidth()
	for i := range bs {
		w = max(w, len(bs[i].URL))
	}

	bold := func(s string) string { return "**" + s + "**" }
	italic := func(s string) string { return "*" + s + "*" }
	bullet := func(header, val string) string { return txt.PaddedLineWithPad(bold("- "+header), val, 12) }

	for i, b := range bs {
		if b.Notes == "" {
			continue
		}

		title := txt.Shorten(b.Title, w)
		if title == "" {
			title = txt.Shorten(b.URL, w)
		}

		tags := txt.TagsWithPound(b.Tags)

		f.Headerln(title).
			Rowln(bullet("ID:", strconv.Itoa(b.ID))).
			Rowln(bullet("Tags:", italic(tags))).
			Rowln(bullet("URL:", b.URL))

		if b.Desc != "" {
			f.Rowln(bullet("Desc:", b.Desc))
		}

		f.Ln().Text(b.Notes)

		if !strings.HasSuffix(b.Notes, "\n") {
			f.Ln()
		}

		// footer
		if i != len(bs)-1 {
			f.Ln().
				Textln("---").
				Ln()
		}
	}

	return c.Print(ctx, f.String())
}

type fieldSpec struct {
	name  string
	limit int // 0: no limit
}

func ByField(ctx context.Context, c *ui.Console, fields string, bs []*bookmark.Bookmark) error {
	parts := strings.Split(fields, ",")
	specs := make([]fieldSpec, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, ":") {
			sub := strings.Split(p, ":")
			specs[i].name = sub[0]
			specs[i].limit, _ = strconv.Atoi(sub[1])
		} else {
			specs[i].name = p
		}
	}

	var buf strings.Builder
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	for _, b := range bs {
		var row []string
		for _, spec := range specs {
			val, err := b.Field(spec.name)
			if err != nil {
				return err
			}
			if spec.limit > 0 {
				val = txt.Shorten(val, spec.limit)
			} else {
				val = txt.Shorten(val, c.MaxWidth()/len(specs))
			}
			row = append(row, val)
		}
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	if err := w.Flush(); err != nil {
		return err
	}

	return c.Print(ctx, buf.String())
}

// DatabasesTable shows a simple table in database information.
func DatabasesTable(ctx context.Context, c *ui.Console, dataPath, defaultName string) error {
	fs, err := files.FindByExtension(dataPath, ".db", ".enc")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	headers := []string{"Name", "Bookmarks", "Tags", "Size", "Path"}
	rows := [][]string{}
	footer := []string{}

	t := strconv.Itoa
	p := c.Palette()
	files.PrioritizeFile(fs, defaultName)

	for _, fpath := range fs {
		dir, fname, ext := filepath.Dir(fpath), filepath.Base(fpath), filepath.Ext(fpath)
		collapsePath := files.CollapseHomeDir(dir)
		cleanName := files.StripExts(fname)
		fsize := files.SizeFormatted(fpath)

		fnameColor := p.BrightBlue.Sprint

		if ext == locker.Extension {
			fnameColor = p.BrightMagenta.Sprint
			cleanName = fnameColor(cleanName)
			rows = append(
				rows,
				[]string{cleanName, "-", "-", fsize, filepath.Join(collapsePath, fnameColor(fname))},
			)
			footer = append(footer, fnameColor(txt.GlyphBlackSquare+" locked"))
			continue
		}

		r, err := db.New(ctx, fpath)
		if err != nil {
			return err
		}
		s := db.NewStats()
		if err := r.Stats(ctx, s); err != nil {
			return err
		}
		s.Name = r.Name()
		r.Close()

		if r.Name() == defaultName {
			fnameColor = p.BrightYellow.With(p.Bold).Sprint
			cleanName = fnameColor(cleanName)
			cleanName += p.Gray.Wrap(" (default)", p.Italic)
			footer = append(footer, fnameColor(txt.GlyphBlackSquare.Prefix(" default")))
		}

		rows = append(
			rows,
			[]string{cleanName, t(s.Bookmarks), t(s.Tags), fsize, filepath.Join(collapsePath, fnameColor(fname))},
		)
	}

	fmt.Fprint(c.Writer(), txt.CreateSimpleTable(headers, rows, strings.Join(footer, " ")))

	return nil
}

// RecordsJSON formats the bookmarks in RecordsJSON.
func RecordsJSON(w io.Writer, bs []*bookmark.Bookmark) error {
	slog.Debug("formatting bookmarks in JSON", "count", len(bs))
	r := make([]*bookmark.BookmarkJSON, 0, len(bs))
	for _, b := range bs {
		r = append(r, b.JSON())
	}

	j, err := port.ToJSON(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Fprintln(w, string(j))

	return nil
}

// TagsJSON formats the tags counter in JSON.
func TagsJSON(ctx context.Context, w io.Writer, p string) error {
	r, err := db.New(ctx, p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	tags, err := r.TagsCounter(ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	j, err := port.ToJSON(tags)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Fprintln(w, string(j))

	return nil
}

// RepoStats prints the database info.
func RepoStats(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	// FIX: Test RepoInfo()
	if err := locker.IsLocked(app.Path.DB()); err != nil {
		sum := dbops.SummaryRepoFromPath(
			ctx,
			d,
			app.Path.DB()+".enc",
			app.Path.Backup(),
		)
		fmt.Fprint(d.Writer(), sum)

		return nil
	}

	if app.Flags.JSON {
		r, err := d.Repository()
		if err != nil {
			return err
		}
		b, err := port.ToJSON(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Fprintln(d.Writer(), string(b))

		return nil
	}

	f := d.Console().Frame()
	f.SetBorders(frame.WithBordersSmallBlock2())

	s, err := dbops.RepoInfo(ctx, d)
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(s)

	g, err := gitops.Info(ctx, d)
	if err != nil {
		return fmt.Errorf("git: %w", err)
	}

	if g != "" {
		sb.WriteString(g)
	}

	fmt.Fprint(d.Writer(), sb.String())

	return nil
}

func Display(ctx context.Context, c *ui.Console, f string, bs []*bookmark.Bookmark) error {
	fm, err := formatter.New(formatter.Format(f))
	if err != nil {
		return err
	}

	return Print(ctx, c, bs, fm.Render)
}

func AppConfig(app *application.App, f *frame.Frame, p *ansi.Palette) error {
	header := func() string {
		return p.BrightBlue.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	const padding = 20
	pad := func(label string, value any) string {
		return txt.PaddedLineWithPad(label+":", value, padding)
	}

	f.CustomFunc(header, app.PrettyVersion()).
		Rowln().
		Rowln(pad("current db", p.BrightYellow.Wrap(app.DBBaseName(), p.Italic))).
		Rowln(pad("format", app.Format))

	// config file
	if files.Exists(app.Path.ConfigFile()) {
		f.Rowln(pad("config:", files.CollapseHomeDir(app.Path.ConfigFile())))
	}

	// menu.
	m := app.Menu
	f.MidCln(p.BrightRed.With(p.Bold), p.BrightRed.Wrap("menu", p.Bold)).
		Rowln(pad("use defaults", boolFmt(m.Defaults))).
		Rowln(pad("format", m.Format)).
		Rowln(pad("prompt", m.Prompt)).
		Rowln(pad("preview enabled", boolFmt(m.Preview))).
		Rowln(pad("header enabled", boolFmt(m.Header.Enabled))).
		Rowln(pad("header separator", m.Header.Sep))

	// keymaps.
	ph := app.Formatter().Menu.Placeholder()
	keymaps := app.Menu.LoadKeymaps(
		menucfg.NewBindBuilder().
			WithCommand(app.Command()).
			WithDBName(app.DBBaseName()).
			WithPlaceholder(ph.Multi()),
	)
	f.MidCln(p.BrightMagenta.With(p.Bold), p.BrightMagenta.Wrap("keymaps", p.Bold))
	for i := range keymaps {
		k := keymaps[i]
		f.Rowln(pad(k.Desc, formatKeymap(p, k)))
	}

	// git.
	if g := app.Git; g.Enabled {
		f.MidCln(p.BrightYellow.With(p.Bold), p.BrightYellow.Wrap("git", p.Bold)).
			Rowln(pad("enabled", boolFmt(g.Enabled))).
			Rowln(pad("logging", boolFmt(g.Log))).
			Rowln(pad("remote", p.Italic.Sprint(g.Remote)))
	}

	f.Flush()

	return nil
}

func formatKeymap(p *ansi.Palette, k *menu.Keymap) string {
	keybind, action, _ := strings.Cut(k.String(), ":")

	status := p.Italic.Sprint(":" + action)
	if k.Hidden {
		status += p.Magenta.Sprint(" hidden")
	}
	if !k.Enabled {
		status += p.Red.Sprint(" disabled")
	}

	return txt.PaddedLineWithPad(
		p.Bold.Sprint(keybind),
		status,
		8,
	)
}

func boolFmt(b bool) string {
	if !b {
		return ansi.BrightRed.Sprint("false")
	}
	return ansi.BrightGreen.Sprint("true")
}
