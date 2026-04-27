package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrURLParamsNotFound = errors.New("params not found")

type ParamsProcessor struct {
	d   *deps.Deps
	app *application.App
	m   *menu.Menu[string]
}

// ParamsURL processes and optionally cleans query params for each bookmark.
func ParamsURL(d *deps.Deps, bs []*bookmark.Bookmark) error {
	app, err := d.Application()
	if err != nil {
		return err
	}

	pp := &ParamsProcessor{
		d:   d,
		app: app,
		m: menu.New[string](
			menu.WithArgs("--cycle"),
			menu.WithBorderLabel("URL Parameters"),
			menu.WithConfig(app.Menu),
			menu.WithHeader("Select with <TAB> which params to remove"),
			menu.WithMultiSelection(),
			menu.WithOutputColor(app.Flags.Color),
		),
	}

	for _, b := range bs {
		if err := processBookmarkParams(pp, b); err != nil {
			return err
		}
	}

	return nil
}

// ParamHighlight returns the URL with its query parameters highlighted with
// the given ansi code.
func ParamHighlight(raw string, color ansi.SGR, styles ...ansi.SGR) string {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return raw
	}

	// Preserve original ordering
	parts := strings.Split(u.RawQuery, "&")

	var highlighted []string
	highlighted = make([]string, 0, len(parts))

	for _, p := range parts {
		if p == "" {
			continue
		}

		kv := strings.SplitN(p, "=", 2)

		if len(kv) == 1 {
			// parameter without value: ?flag
			highlighted = append(highlighted, color.Wrap(kv[0], styles...))
			continue
		}

		key := kv[0]
		val := kv[1]

		colored := color.Wrap(key+"="+val, styles...)
		highlighted = append(highlighted, colored)
	}

	// rebuild manually so we don't lose encoding or formatting
	var sb strings.Builder
	sb.Grow(len(raw) + len(parts)*10)

	// base URL without query
	base := raw[:strings.Index(raw, "?")+1]
	sb.WriteString(base)
	sb.WriteString(strings.Join(highlighted, "&"))

	return sb.String()
}

// PrintParamChanges displays the original and cleaned URL with removed
// parameters.
func diffParams(d *deps.Deps, originalURL string, params []string) int {
	f, p := d.Console().Frame(), d.Console().Palette()
	f.Headerln(p.Bold.Wrap("Cleaning URL parameters", p.Yellow))

	f.Midln("Original URL:").Rowln(" " + p.Dim.Sprint(txt.Shorten(originalURL, d.Console().MaxWidth()))).
		Rowln().Midln(fmt.Sprintf("Found %d: ", len(params)))

	// key=value params
	sep := "="
	for i := range params {
		values := strings.Split(params[i], sep)
		f.Rowln(p.BrightRed.Sprint(values[0]) + p.Dim.Sprint(sep+strings.TrimSpace(values[1])))
	}

	f.Rowln()

	lines := txt.CountLines(f.String())
	if d.App.Flags.Yes {
		lines--
	}

	return lines
}

// paramsCleaner removes all query parameters from the URL and returns the
// cleaned string.
func paramsCleaner(u *url.URL) string {
	q := u.Query()
	for key := range u.Query() {
		q.Del(key)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// selectParams presents a multi-select menu to choose which query params to
// remove.
func selectParams(m *menu.Menu[string], u *url.URL) ([]string, error) {
	query := u.Query()
	sep := "="
	items := make([]string, 0, len(query))
	for key, values := range query {
		for _, value := range values {
			items = append(items, fmt.Sprintf("%s%s%s\n", ansi.Red.Sprint(key), sep, value))
		}
	}

	slog.Debug("params selection", slog.Int("params", len(items)))
	if len(items) == 0 {
		return nil, ErrNoItems
	}

	m.UpdatePreview(fmt.Sprintf("echo %q", u.String()))
	params, err := m.Select(items)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(params))
	for i := range params {
		result = append(result, ansi.Remover(strings.TrimSpace(params[i])))
	}

	return result, nil
}

// processBookmarkParams prompts for param removal and persists updates if
// confirmed.
func processBookmarkParams(pp *ParamsProcessor, b *bookmark.Bookmark) error {
	u, err := url.Parse(b.URL)
	if err != nil {
		return err
	}

	if len(u.Query()) == 0 {
		slog.Debug("skipping: no params found", slog.String("url", b.URL))
		return nil
	}

	opt, linesToClear, err := promptParamRemoval(pp.d, b.URL, u.Query())
	if err != nil {
		return err
	}

	// clear the terminal lines after receiving input
	pp.d.Console().ClearLine(linesToClear)

	newURL, skipped, err := computeNewURL(pp.m, u, opt)
	if err != nil {
		return err
	}

	if skipped {
		p := pp.d.Console().Palette()
		id := func(val any) string { return p.Bold.Sprint("[", val, "] ") }
		pp.d.Console().Frame().Warning(id(b.ID) + p.BrightYellow.Wrap("skipping", p.Italic) + " bookmark").Ln().Flush()
		return nil
	}

	// ignore if new bookmark
	if b.ID == 0 {
		b.URL = newURL
		return nil
	}

	// save to db and git
	return persistBookmarkUpdate(pp, b, newURL)
}

// promptParamRemoval displays URL param diff and asks whether to remove them.
func promptParamRemoval(d *deps.Deps, urlStr string, q url.Values) (opt string, lines int, err error) {
	c := d.Console()
	f, p := c.Frame(), c.Palette()
	sep := "="

	// Format parameters for the diff
	params := make([]string, 0, len(q))
	for key, values := range q {
		for _, value := range values {
			params = append(params, fmt.Sprintf("%s%s%s\n", p.Red.Sprint(key), sep, value))
		}
	}

	lines = diffParams(d, urlStr, params)
	f.Flush()

	opts := []string{"no", "yes", "select"}
	opt, err = c.Choose(fmt.Sprintf("remove %d params?", len(q)), opts, "n")

	return
}

// computeNewURL returns a new URL based on the selected option or reports
// skip/invalid choice.
func computeNewURL(m *menu.Menu[string], u *url.URL, opt string) (newURL string, skipped bool, err error) {
	switch strings.ToLower(opt) {
	case "n", "no":
		return "", true, nil
	case "y", "yes":
		return paramsCleaner(u), false, nil
	case "s", "select":
		selected, err := selectParams(m, u)
		if err != nil {
			return "", false, err
		}

		q := u.Query()
		for _, param := range selected {
			parts := strings.SplitN(param, "=", 2)
			if len(parts) > 0 {
				q.Del(parts[0])
			}
		}

		u.RawQuery = q.Encode()

		return u.String(), false, nil
	default:
		return "", false, ErrInvalidOption
	}
}

// persistBookmarkUpdate updates the bookmark URL in the DB and Git if no
// duplicate exists.
func persistBookmarkUpdate(pp *ParamsProcessor, b *bookmark.Bookmark, newURL string) error {
	c := pp.d.Console()
	f, p := c.Frame(), c.Palette()
	id := func(val any) string { return p.Bold.Sprint("[", val, "] ") }

	newB := *b
	newB.URL = newURL

	// check for duplicates
	if book, has := pp.d.Repo.Has(pp.d.Context(), newB.URL); has {
		f.Error(id(newB.ID) + p.BrightRed.Wrap("already", p.Italic) + " exists with " + id(book.ID)).
			Ln().Flush()
		return nil
	}

	f.Success(id(newB.ID) + p.BrightGreen.Wrap("updating", p.Italic) + " bookmark").Ln().Flush()
	if err := pp.d.Repo.UpdateOne(pp.d.Context(), &newB); err != nil {
		return err
	}

	return git.UpdateBookmark(pp.app, b, &newB)
}
