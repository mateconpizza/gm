package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrURLParamsNotFound = errors.New("params not found")

// ParamsURL processes and optionally cleans query params for each bookmark.
func ParamsURL(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return ErrNoItems
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	m := picker.New[string](
		app,
		menu.WithArgs("--cycle"),
		menu.WithBorderLabel("URL Parameters"),
		menu.WithHeader("Select with <TAB> which params to remove"),
		menu.WithMultiSelection(),
	)

	for _, b := range bs {
		newURL, err := ProcessBookmarkParams(ctx, d, m, b.URL)
		if err != nil {
			return err
		}

		if newURL == "" {
			continue
		}

		b.URL = newURL

		// save to db and git
		if err := persistBookmarkUpdate(ctx, d, b, newURL); err != nil {
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
func diffParams(ctx context.Context, d *deps.Deps, originalURL string, params []string) (int, error) {
	f, p := d.Console().Frame(), d.Console().Palette()
	header := func() string { return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold) }
	subtitle := p.Dim.With(p.Italic).
		Sprint("query parameters detected in the URL")

	f.CustomFunc(header, p.Bold.Wrap("Clean URL parameters", p.Yellow)).Ln().
		Midln(subtitle).
		Rowln().
		Midln("Original URL:").
		Rowln(" " + p.Dim.Sprint(txt.Shorten(originalURL, d.Console().MaxWidth()))).
		Rowln().
		Midln(fmt.Sprintf("Parameters to remove (%d) ", len(params)))

	// key=value params
	sep := "="
	for i := range params {
		values := strings.Split(params[i], sep)
		k, v := values[0], txt.Shorten(values[1], d.Console().Term().MinWidth())
		f.Rowln(p.BrightRed.Sprint(k) + p.Dim.Sprint(sep+strings.TrimSpace(v)))
	}

	f.Rowln()

	lines := txt.CountLines(f.String())
	app, err := d.Application(ctx)
	if err != nil {
		return 0, err
	}
	if app.Flags.Yes {
		lines--
	}

	return lines, nil
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

// ProcessBookmarkParams prompts for param removal and persists updates if
// confirmed.
func ProcessBookmarkParams(ctx context.Context, d *deps.Deps, m *menu.Menu[string], urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	if len(u.Query()) == 0 {
		slog.Debug("skipping: no params found", slog.String("url", urlStr))
		return "", nil
	}

	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	if app.Flags.Yes || app.Flags.Force {
		return paramsCleaner(u), nil
	}

	opt, linesToClear, err := promptParamRemoval(ctx, d, urlStr, u.Query())
	if err != nil {
		return "", err
	}

	// clear the terminal lines after receiving input
	d.Console().ClearLine(linesToClear)

	newURL, skipped, err := computeNewURL(m, u, opt)
	if err != nil {
		return "", err
	}

	if skipped {
		f, p := d.Console().Frame(), d.Console().Palette()
		f.Warning(p.BrightYellow.Wrap("skipping ", p.Italic) + p.Dim.Sprint(urlStr)).Ln().Flush()
		return "", nil
	}

	return newURL, nil
}

// promptParamRemoval displays URL param diff and asks whether to remove them.
func promptParamRemoval(
	ctx context.Context,
	d *deps.Deps,
	urlStr string,
	q url.Values,
) (opt string, lines int, err error) {
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

	lines, err = diffParams(ctx, d, urlStr, params)
	if err != nil {
		return "", 0, err
	}
	f.Flush()

	opts := []string{"no", "yes"}
	if len(q) > 1 {
		opts = append(opts, "select")
	}
	opt, err = c.Choose(ctx, p.BrightRed.Wrap("continue?", p.Bold), opts, "n")

	return opt, lines, err
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
func persistBookmarkUpdate(ctx context.Context, d *deps.Deps, b *bookmark.Bookmark, newURL string) error {
	c := d.Console()
	f, p := c.Frame(), c.Palette()
	id := func(val any) string { return p.Bold.Sprint("[", val, "] ") }

	newB := *b
	newB.URL = newURL

	// check for duplicates
	r, err := d.Repository()
	if err != nil {
		return err
	}
	// TODO: use bookmark.Deduplicate or port.DeduplicateReport
	if book, has := r.Has(ctx, newB.URL); has {
		f.Error(id(newB.ID) + p.BrightRed.Wrap("already", p.Italic) + " exists with " + id(book.ID)).
			Ln().Flush()
		return nil
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if err := r.UpdateOne(ctx, &newB); err != nil {
		return err
	}

	return gitops.Update(ctx, app, b, &newB)
}
