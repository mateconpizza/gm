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
	bullet := func(header, val string) string { return txt.PaddedLineWithWidth(bold("- "+header), val, 12) }

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

type FileInfo struct {
	Path string // Database file path
	Name string // Database name with ext
	Base string // Database name without ext
	Ext  string
	Size string
	Skip bool
}

type FileFilterFunc func(info *FileInfo) bool

type FileDecorator func(info *FileInfo) *FileInfo

type FileListPreprocessor func(paths []string) []string

type DatabasesTableOpt struct {
	decorators    []FileDecorator
	filters       []FileFilterFunc
	preprocessors []FileListPreprocessor
	footer        []string
}

type FileTableOptFunc func(*DatabasesTableOpt)

func WithDecorator(fn func(info *FileInfo) *FileInfo) FileTableOptFunc {
	return func(o *DatabasesTableOpt) { o.decorators = append(o.decorators, fn) }
}

func WithFilter(fn FileFilterFunc) FileTableOptFunc {
	return func(o *DatabasesTableOpt) { o.filters = append(o.filters, fn) }
}

func WithPreprocessor(fn func(paths []string) []string) FileTableOptFunc {
	return func(o *DatabasesTableOpt) { o.preprocessors = append(o.preprocessors, fn) }
}

func WithFooter(foo []string) FileTableOptFunc {
	return func(o *DatabasesTableOpt) { o.footer = foo }
}

func (o *DatabasesTableOpt) include(info *FileInfo) (*FileInfo, bool) {
	for _, fn := range o.decorators {
		info = fn(info)
	}

	if info.Skip {
		return info, false
	}

	for _, fn := range o.filters {
		if !fn(info) {
			return info, false
		}
	}

	return info, true
}

func (o *DatabasesTableOpt) preprocess(paths []string) []string {
	for _, fn := range o.preprocessors {
		paths = fn(paths)
	}
	return paths
}

// DatabasesTable shows a simple table in database information.
func DatabasesTable(ctx context.Context, w io.Writer, dataPath string, opts ...FileTableOptFunc) error {
	o := &DatabasesTableOpt{}
	for _, opt := range opts {
		opt(o)
	}

	fs, err := files.FindByExtension(dataPath, ".db", ".enc")
	if err != nil {
		return err
	}

	fs = o.preprocess(fs)

	headers := []string{"Name", "Records", "Tags", "Size", "Path"}
	rows := [][]string{}
	toStr := strconv.Itoa

	for _, fpath := range fs {
		info := &FileInfo{
			Path: fpath,
			Name: filepath.Base(fpath),
			Base: filepath.Base(fpath),
			Ext:  filepath.Ext(fpath),
			Size: files.SizeFormatted(fpath),
		}

		info, ok := o.include(info)
		if !ok {
			continue
		}

		h := files.CollapseHomeDir(filepath.Dir(fpath))

		if info.Ext == locker.Extension.String() {
			rows = append(
				rows,
				[]string{info.Base, "-", "-", info.Size, filepath.Join(h, info.Name)},
			)
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
		r.Close()

		rows = append(
			rows,
			[]string{info.Base, toStr(s.Bookmarks), toStr(s.Tags), info.Size, filepath.Join(h, info.Name)},
		)
	}

	fmt.Fprint(w, txt.CreateSimpleTable(headers, rows, o.footer...))
	return nil
}

func DatabaseDecorator(app *application.App) func(info *FileInfo) *FileInfo {
	gm, _ := gitops.NewManager(&gitops.ManagerConfig{
		Root: app.Path.Git(),
	})

	p := ansi.NewPalette(app.Flags.Color)
	g := app.Glyphs()

	return func(info *FileInfo) *FileInfo {
		baseName := files.StripExts(info.Base)
		isDefault := baseName == app.DBBaseName()
		isTracked := gm.IsEnabled() && gm.IsTracked(baseName)
		isLocked := filepath.Ext(info.Base) == locker.Extension.String()

		var sb strings.Builder
		var status strings.Builder

		if isTracked {
			status.WriteByte(' ')
			status.WriteString(p.BrightYellow.Sprint(g.Git))
		}

		if isDefault {
			sb.WriteString(p.BrightYellow.Sprint(baseName))
			sb.WriteString(p.Gray.Wrap(" (default)", p.Italic))
			sb.WriteString(status.String())

			info.Name = p.BrightYellow.Sprint(info.Base)
			info.Base = sb.String()

			return info
		}

		if isLocked {
			sb.WriteString(baseName)

			status.WriteByte(' ')
			status.WriteString(p.BrightMagenta.Sprint(g.GPG))

			sb.WriteString(status.String())
			info.Name = p.BrightMagenta.Sprint(info.Base)
			info.Base = sb.String()
			return info
		}

		sb.WriteString(baseName)
		sb.WriteString(status.String())

		info.Name = p.BrightBlue.Sprint(info.Base)
		info.Base = sb.String()

		return info
	}
}

func TimestampDecorator(app *application.App) func(info *FileInfo) *FileInfo {
	p := ansi.NewPalette(app.Flags.Color)

	return func(info *FileInfo) *FileInfo {
		name := p.Remover(info.Name)
		t, _, _ := strings.Cut(name, "_")
		rt := txt.RelativeTime(t)
		if rt != "invalid timestamp" {
			info.Base += " " + p.Gray.Wrap(rt, p.Italic)
		}
		return info
	}
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

type GitInfoFunc func(ctx context.Context, d *deps.Deps) (string, error)

// RepoStats prints the database info.
func RepoStats(ctx context.Context, d *deps.Deps, gitInfo GitInfoFunc) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	// FIX: Test RepoInfo()
	if err := locker.IsLocked(app.Path.DB()); err != nil {
		sum := dbops.SummaryRepoFromPath(
			ctx,
			d.Console(),
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
			return err
		}

		fmt.Fprintln(d.Writer(), string(b))

		return nil
	}

	f := d.Console().Frame()
	f.SetBorders(frame.WithBordersSmallBlock2())

	s, err := dbops.RepoInfo(ctx, d)
	if err != nil {
		return fmt.Errorf("info: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(s)

	g, err := gitInfo(ctx, d)
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

func AppConfig(ctx context.Context, app *application.App, c *ui.Console) error {
	f, p := c.Frame(), c.Palette()
	header := func() string {
		return p.BrightBlue.Wrap(c.Glyphs().Square+" ", p.Bold)
	}

	const padding = 20
	pad := func(label string, value any) string {
		return txt.PaddedLineWithWidth(label+":", value, padding)
	}

	f.CustomFunc(header, app.PrettyVersion()).
		Rowln().
		Rowln(pad("db", p.BrightYellow.Wrap(app.DBBaseName(), p.Italic))).
		Rowln(pad("format", app.Format())).
		Rowln(pad("glyphs", app.UI.GlyphMode))

	// config file
	if files.Exists(app.Path.ConfigFile()) {
		f.Rowln(pad("config", files.CollapseHomeDir(app.Path.ConfigFile())))
	}

	boolFmt := func(b bool) string {
		if !b {
			return p.BrightRed.Sprint("false")
		}
		return p.BrightGreen.Sprint("true")
	}

	// menu.
	m := app.Menu
	f.MidCln(p.BrightRed.With(p.Bold).Sprint, p.BrightRed.Wrap("menu", p.Bold)).
		Rowln(pad("use defaults", boolFmt(m.Defaults))).
		Rowln(pad("format", m.Format)).
		Rowln(pad("prompt", fmt.Sprintf("%q", m.Prompt))).
		Rowln(pad("preview enabled", boolFmt(m.Preview))).
		Rowln(pad("header enabled", boolFmt(m.Header.Enabled))).
		Rowln(pad("header separator", fmt.Sprintf("%q", m.Header.Sep)))

	// keymaps.
	ph := app.Formatter().Menu.Placeholder()
	keymaps := app.Menu.LoadKeymaps(
		menucfg.NewBindBuilder().
			WithCommand(app.Command()).
			WithDBName(app.DBBaseName()).
			WithPlaceholder(ph.Multi()),
	)
	f.MidCln(p.BrightMagenta.With(p.Bold).Sprint, p.BrightMagenta.Wrap("keymaps", p.Bold))
	for i := range keymaps {
		k := keymaps[i]
		f.Rowln(pad(k.Desc, formatKeymap(p, k)))
	}

	// git.
	if g := app.Git; g.Enabled {
		f.MidCln(p.BrightYellow.With(p.Bold).Sprint, p.BrightYellow.Wrap("git", p.Bold)).
			Rowln(pad("enabled", boolFmt(g.Enabled))).
			Rowln(pad("logging", boolFmt(g.Log))).
			Rowln(pad("remote", p.Italic.Sprint(g.Remote)))
	}

	return c.Print(ctx, f.String())
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

	return txt.PaddedLineWithWidth(
		p.Bold.Sprint(keybind),
		status,
		8,
	)
}
