// Package gecko provides functionalities for importing bookmarks from
// Gecko-based web browsers like Firefox.
package gecko

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mateconpizza/rotato"
	ini "gopkg.in/ini.v1"

	browserpath "github.com/mateconpizza/gm/internal/sys/browser/paths"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var ignoredPrefixes = []string{
	"about:",
	"apt:",
	"chrome://",
	"file://",
	"place:",
	"vivaldi://",
	"moz-extension://",
}

var (
	ErrBrowserIsOpen           = errors.New("browser is open")
	ErrBrowserConfigPathNotSet = errors.New("browser config path not set")
	ErrBrowserUnsupported      = errors.New("browser is unsupported")
	ErrDatabaseLocked          = errors.New("database is locked")
)

func getTodayFormatted() string {
	today := time.Now()
	return today.Format("2006Jan02")
}

var geckoBrowserPaths = map[string]Paths{
	"Firefox": {
		profiles:  browserpath.GeckoProfilePath(".mozilla/firefox"),
		bookmarks: browserpath.GeckoBookmarkPath(".mozilla/firefox"),
	},
	"Zen": {
		profiles:  browserpath.GeckoProfilePath(".zen"),
		bookmarks: browserpath.GeckoBookmarkPath(".zen"),
	},
	"Waterfox": {
		profiles:  browserpath.GeckoProfilePath(".waterfox"),
		bookmarks: browserpath.GeckoBookmarkPath(".waterfox"),
	},
}

type Paths struct {
	profiles  string
	bookmarks string
}

type GeckoBrowser struct {
	name  string
	short string
	color ansi.SGR
	paths Paths
}

func (b *GeckoBrowser) Name() string {
	return b.name
}

func (b *GeckoBrowser) Short() string {
	return b.short
}

func (b *GeckoBrowser) Color(s string) string {
	return b.color.Sprint(s)
}

func (b *GeckoBrowser) LoadPaths() error {
	p, ok := geckoBrowserPaths[b.name]
	if !ok {
		return fmt.Errorf("%w: %q", ErrBrowserUnsupported, b.name)
	}

	b.paths = p

	return nil
}

func (b *GeckoBrowser) Import(ctx context.Context, c *ui.Console, force bool) ([]*bookmark.Bookmark, error) {
	paths := b.paths
	if paths.profiles == "" || paths.bookmarks == "" {
		return nil, ErrBrowserConfigPathNotSet
	}

	if !files.Exists(paths.profiles) {
		return nil, fmt.Errorf("%w: %q", files.ErrFileNotFound, paths.profiles)
	}

	profiles, err := allProfiles(paths.profiles)
	if err != nil {
		return nil, err
	}

	f, p := c.Frame(), c.Palette()
	f.Rowln().
		Header(fmt.Sprintf("Starting %s import\n", b.Color(b.Name()))).
		Midln("Found " + p.Bold.Sprint(len(profiles)) + " profiles").
		Flush()

	var bs []*bookmark.Bookmark
	for profile, v := range profiles {
		p := fmt.Sprintf(paths.bookmarks, v)
		processProfile(ctx, c, &bs, profile, p, force)
	}

	return bs, nil
}

func New(name string, c ansi.SGR) *GeckoBrowser {
	return &GeckoBrowser{
		name:  name,
		short: strings.ToLower(string(name[0])),
		color: c,
	}
}

type geckoBookmark struct {
	FK     int    `db:"fk"`
	Parent int    `db:"parent"`
	Title  string `db:"title"`
	URL    string `db:"url"`
	Tags   string `db:"tags"`
}

// openSQLite opens the SQLite database and returns a *sql.DB object.
func openSQLite(ctx context.Context, c *ui.Console, dbPath string) (*sqlx.DB, error) {
	cfg, err := db.NewSQLiteCfg(fmt.Sprintf("file:%s?cache=shared", dbPath))
	if err != nil {
		return nil, err
	}

	s := rotato.New(
		rotato.WithMessage(c.Palette().BrightBlue.Sprint("connecting to database...")),
		rotato.WithSpinnerColor(rotato.FgGray),
		rotato.WithFailMessageColor(rotato.FgBrightRed),
	)
	s.Start(ctx)
	defer s.Done()

	r, err := db.OpenDatabase(ctx, dbPath, cfg)
	if err != nil {
		if strings.Contains(err.Error(), ErrDatabaseLocked.Error()) {
			return nil, ErrBrowserIsOpen
		}

		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return r, nil
}

// isNonGenericURL checks if the given URL is a non-generic URL based on a set
// of ignored prefixes.
func isNonGenericURL(url string) bool {
	return slices.ContainsFunc(ignoredPrefixes, func(prefix string) bool {
		return strings.HasPrefix(url, prefix)
	})
}

// queryBookmarks queries the bookmarks table to retrieve some sample data.
func queryBookmarks(r *sqlx.DB) ([]*geckoBookmark, error) {
	q := "SELECT DISTINCT fk, parent, title FROM moz_bookmarks WHERE type=1 AND title IS NOT NULL"
	var bs []*geckoBookmark

	err := r.Select(&bs, q)
	if err != nil {
		return nil, fmt.Errorf("failed to query bookmarks: %w", err)
	}

	parseGeckoBookmark := func(gb *geckoBookmark) error {
		gb.URL, err = processURLs(r, gb.FK)
		if err != nil {
			return err
		}

		if gb.URL == "" {
			return nil
		}

		gb.Tags, err = processTags(r, gb.FK)
		if err != nil {
			return err
		}

		gb.Tags += " " + getTodayFormatted()
		gb.Tags = bookmark.ParseTags(gb.Tags)

		return nil
	}

	for _, b := range bs {
		if err := parseGeckoBookmark(b); err != nil {
			return nil, err
		}
	}

	return bs, nil
}

func allProfiles(p string) (map[string]string, error) {
	inidata, err := ini.Load(p)
	if err != nil {
		return nil, fmt.Errorf("error loading file: %w", err)
	}

	result := make(map[string]string)

	sections := inidata.Sections()
	for _, sec := range sections {
		if !strings.HasPrefix(sec.Name(), "Profile") {
			continue
		}

		name := sec.Key("Name").String()
		path := sec.Key("Path").String()
		result[name] = path
	}

	return result, nil
}

// processProfile processes a single profile and extracts bookmarks.
func processProfile(ctx context.Context, c *ui.Console, bs *[]*bookmark.Bookmark, profile, path string, force bool) {
	if !confirmImport(ctx, c, profile, force) {
		return
	}

	p := c.Palette()
	path = files.ExpandHomeDir(path)
	r, err := openSQLite(ctx, c, path)
	if err != nil {
		handleDBError(c, p, profile, err)
		return
	}

	defer func() {
		if r == nil {
			return
		}
		_ = r.Close()
		slog.Debug("database for profile closed", "profile", profile)
	}()

	gmarks, err := queryBookmarks(r)
	if err != nil {
		fmt.Fprintf(c.Writer(), "err querying bookmarks for profile %q: %v\n", profile, err)
		return
	}

	skipped := importBookmarks(bs, gmarks)
	if err := r.Close(); err != nil {
		slog.Error("closing rows", "err", err)
	}

	found := p.BrightBlue.Sprint("found")
	c.Info(fmt.Sprintf("%s %d bookmarks\n", found, len(*bs)-skipped)).Flush()
}

func confirmImport(ctx context.Context, c *ui.Console, profile string, force bool) bool {
	p := c.Palette()
	if force {
		c.Warning("force import bookmarks from '" + profile + "' profile\n").Flush()
		return true
	}

	if err := c.ConfirmErr(ctx, fmt.Sprintf("import bookmarks from %q profile?", profile), "y"); err != nil {
		c.ClearLine(1)
		pf := p.Italic.Wrap(profile, p.Bold)
		c.Warning(p.BrightYellow.Wrap("skipping", p.Italic) + " profile " + pf).Ln().Flush()
		return false
	}

	return true
}

func handleDBError(c *ui.Console, p *ansi.Palette, profile string, err error) {
	slog.Error("opening database for profile", "profile", profile, "err", err)
	if errors.Is(err, ErrBrowserIsOpen) {
		c.Error("database is " + p.BrightRed.Sprint("locked") + ", maybe browser is open?\n").Flush()
		return
	}
	fmt.Fprintf(c.Writer(), "err opening database for profile %q: %v\n", profile, err)
}

func importBookmarks(bs *[]*bookmark.Bookmark, gmarks []*geckoBookmark) int {
	skipped := 0
	for _, gb := range gmarks {
		if gb.URL == "" {
			continue
		}
		b := bookmark.New()
		b.Title = gb.Title
		b.URL = gb.URL
		b.Tags = gb.Tags

		if isDuplicate(*bs, b.URL) {
			skipped++
			continue
		}
		*bs = append(*bs, b)
	}
	return skipped
}

func isDuplicate(existing []*bookmark.Bookmark, url string) bool {
	for _, b := range existing {
		if b.URL == url {
			return true
		}
	}
	return false
}

// processTags processes the tags for a single bookmark.
func processTags(r *sqlx.DB, fk int) (string, error) {
	var (
		tagIDs []int
		tags   []string
	)
	// fetch all parent tag ids in a single query
	err := r.Select(&tagIDs, "SELECT parent FROM moz_bookmarks WHERE fk=? AND title IS NULL", fk)
	if err != nil {
		return "", fmt.Errorf("failed to query tags: %w", err)
	}

	if len(tagIDs) == 0 {
		return "", nil
	}
	// fetch all tag titles in a single query using in clause
	query, args, err := sqlx.In("SELECT title FROM moz_bookmarks WHERE id IN (?)", tagIDs)
	if err != nil {
		return "", fmt.Errorf("failed to build IN query: %w", err)
	}

	query = r.Rebind(query)

	err = r.Select(&tags, query, args...)
	if err != nil {
		return "", fmt.Errorf("failed to query tag titles: %w", err)
	}

	if len(tags) == 0 {
		return "", nil
	}

	return strings.Join(tags, ","), nil
}

func processURLs(r *sqlx.DB, fk int) (string, error) {
	var url string

	rows, err := r.Query("SELECT url FROM moz_places where id=?", fk)
	if err != nil {
		return url, fmt.Errorf("failed to query places: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("closing rows", "err", err)
		}
	}()

	for rows.Next() {
		if err := rows.Scan(&url); err != nil {
			return url, fmt.Errorf("failed to scan url row: %w", err)
		}

		if isNonGenericURL(url) {
			return "", nil
		}
	}

	if err := rows.Err(); err != nil {
		return url, fmt.Errorf("rows iteration error: %w", err)
	}

	return url, nil
}
