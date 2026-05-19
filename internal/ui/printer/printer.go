// Package printer provides functions to format and print bookmark data,
// including records, tags, and repository information.
package printer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrInvalidFormat = errors.New("invalid format")
	ErrUnknownFormat = errors.New("unknown format")
)

func MenuPreview(c *ui.Console, bs []*bookmark.Bookmark, f string) error {
	fm, err := formatter.New(f)
	if err != nil {
		return err
	}

	for i := range bs {
		fmt.Print(fm.Render(c, bs[i]))
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
			buf.WriteString("\n")
		}
	}

	return c.Term().Print(ctx, buf.String())
}

// TagsList lists the tags.
func TagsList(ctx context.Context, p string) error {
	r, err := db.New(ctx, p)
	if err != nil {
		return err
	}
	defer r.Close()

	tags, err := db.TagsList(ctx, r)
	if err != nil {
		return fmt.Errorf("tagslist: %w", err)
	}

	fmt.Println(strings.Join(tags, "\n"))

	return nil
}

// Print formats the bookmarks in oneline.
func Print(c *ui.Console, bs []*bookmark.Bookmark, fn func(*ui.Console, *bookmark.Bookmark) string) error {
	var buf strings.Builder
	for i := range bs {
		buf.WriteString(fn(c, bs[i]))
	}

	return c.Term().Print(context.Background(), buf.String())
}

// Notes formats the bookmarks notes.
func Notes(c *ui.Console, bs []*bookmark.Bookmark) error {
	printed := false
	for _, b := range bs {
		if b.Notes == "" {
			continue
		}
		if printed {
			fmt.Println()
		}
		fmt.Print(formatter.Notes(c, b))
		printed = true
	}
	return nil
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

	return c.Term().Print(ctx, buf.String())
}

// DatabasesTable shows a simple table in database information.
func DatabasesTable(ctx context.Context, c *ui.Console, dataPath, defaultName string) error {
	fs, err := files.FindByExtList(dataPath, ".db", ".enc")
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
		cleanName := files.StripSuffixes(fname)
		fsize := files.SizeFormatted(fpath)

		fnameColor := p.BrightBlue.Sprint

		if ext == locker.Extension {
			fnameColor = p.BrightMagenta.Sprint
			cleanName = fnameColor(cleanName)
			rows = append(
				rows,
				[]string{cleanName, "-", "-", fsize, filepath.Join(collapsePath, fnameColor(fname))},
			)
			footer = append(footer, fnameColor(txt.UnicodeBlackSquare+" locked"))
			continue
		}

		r, err := db.New(ctx, fpath)
		if err != nil {
			return err
		}
		s, err := db.NewStats(ctx, r)
		if err != nil {
			return err
		}
		r.Close()

		if r.Name() == defaultName {
			fnameColor = p.BrightYellow.With(p.Bold).Sprint
			cleanName = fnameColor(cleanName)
			cleanName += p.Gray.Wrap(" (default)", p.Italic)
			footer = append(footer, fnameColor(txt.UnicodeBlackSquare+" default"))
		}

		rows = append(
			rows,
			[]string{cleanName, t(s.Bookmarks), t(s.Tags), fsize, filepath.Join(collapsePath, fnameColor(fname))},
		)
	}

	fmt.Print(txt.CreateSimpleTable(headers, rows, strings.Join(footer, " ")))

	return nil
}

// RecordsJSON formats the bookmarks in RecordsJSON.
func RecordsJSON(bs []*bookmark.Bookmark) error {
	slog.Debug("formatting bookmarks in JSON", "count", len(bs))
	r := make([]*bookmark.BookmarkJSON, 0, len(bs))
	for _, b := range bs {
		r = append(r, b.JSON())
	}

	j, err := port.ToJSON(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(string(j))

	return nil
}

// TagsJSON formats the tags counter in JSON.
func TagsJSON(ctx context.Context, p string) error {
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

	fmt.Println(string(j))

	return nil
}

// RepoInfo prints the database info.
func RepoInfo(d *deps.Deps) error {
	app, err := d.Application()
	if err != nil {
		return err
	}
	// FIX: Test RepoInfo()
	if err := locker.IsLocked(app.Path.Database); err != nil {
		fmt.Print(
			summary.RepoFromPath(
				d,
				app.Path.Database+".enc",
				app.Path.Backup,
			),
		)

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

		fmt.Println(string(b))

		return nil
	}

	var sb strings.Builder

	s, err := summary.Info(d)
	if err != nil {
		return err
	}
	sb.WriteString(s)

	g, err := git.Info(
		d.Console(),
		app.Path.Database,
		app.Git,
	)
	if err != nil {
		return fmt.Errorf("git: %w", err)
	}

	if g != "" {
		sb.WriteString(g)
	}

	fmt.Print(sb.String())

	return nil
}

func Display(c *ui.Console, f string, bs []*bookmark.Bookmark) error {
	fm, err := formatter.New(f)
	if err != nil {
		return err
	}

	return Print(c, bs, fm.Render)
}
