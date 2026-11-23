// Package gecko provides functionalities for importing bookmarks from
// Gecko-based web browsers like Firefox.
package gecko

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	menu "github.com/mateconpizza/go-fzf"
	files "github.com/mateconpizza/gofiles"
	"github.com/mateconpizza/rotato"
	ini "gopkg.in/ini.v1"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/sys/browser"
	browserpath "github.com/mateconpizza/gm/internal/sys/browser/paths"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var (
	ErrBrowserIsOpen           = errors.New("browser is open")
	ErrBrowserConfigPathNotSet = errors.New("browser config path not set")
	ErrDatabaseLocked          = errors.New("database is locked")
)

const Gecko browser.Engine = "gecko"

var _ browser.Browser = (*GeckoBrowser)(nil)

var ignoredPrefixes = []string{
	"about:",
	"apt:",
	"chrome://",
	"file://",
	"place:",
	"vivaldi://",
	"moz-extension://",
}

var Supported = []browser.Supported{
	{Browser: New("Firefox")},
	{Browser: New("Zen")},
	{Browser: New("Waterfox")},
}

var geckoBrowserPaths = map[string]Paths{
	"Firefox": {
		profiles: []string{
			browserpath.GeckoProfilePath(".mozilla/firefox"),
			browserpath.GeckoProfilePath(".config/mozilla/firefox"),
		},
		bookmarks: []string{
			browserpath.GeckoBookmarkPath(".mozilla/firefox"),
			browserpath.GeckoBookmarkPath(".config/mozilla/firefox"),
		},
	},
	"Zen": {
		profiles: []string{
			browserpath.GeckoProfilePath(".zen"),
			browserpath.GeckoProfilePath(".config/zen"),
		},
		bookmarks: []string{
			browserpath.GeckoBookmarkPath(".zen"),
			browserpath.GeckoBookmarkPath(".config/zen"),
		},
	},
	"Waterfox": {
		profiles: []string{
			browserpath.GeckoProfilePath(".waterfox"),
			browserpath.GeckoProfilePath(".config/waterfox"),
		},
		bookmarks: []string{
			browserpath.GeckoBookmarkPath(".waterfox"),
			browserpath.GeckoBookmarkPath(".config/waterfox"),
		},
	},
}

func Register(name, path string) *GeckoBrowser {
	name = txt.Capitalize(name)

	p := Paths{
		profiles: []string{
			browserpath.GeckoProfilePath(path),
		},
		bookmarks: []string{
			browserpath.GeckoBookmarkPath(path),
		},
	}

	geckoBrowserPaths[name] = p
	return &GeckoBrowser{
		name:  name,
		paths: p,
	}
}

type Paths struct {
	profiles  []string
	bookmarks []string
}

type GeckoBrowser struct {
	name  string
	paths Paths
}

func New(name string) *GeckoBrowser {
	return &GeckoBrowser{name: name}
}

func (b *GeckoBrowser) Name() string           { return b.name }
func (b *GeckoBrowser) String() string         { return ansi.BrightRed.Sprint(b.name) }
func (b *GeckoBrowser) Engine() browser.Engine { return Gecko }

func (b *GeckoBrowser) LoadPaths(l *slog.Logger) error {
	log := l.With("cmp", "gecko", "fn", "load-paths", "browser", b.name)
	log.Debug("loading browser paths")

	p, ok := geckoBrowserPaths[b.name]
	if !ok {
		err := fmt.Errorf("%w: %q", browser.ErrUnsupported, b.name)
		log.Warn("unsupported browser", "error", err)
		return err
	}
	b.paths = p
	log.Debug("browser paths loaded")
	return nil
}

func (b *GeckoBrowser) Import(ctx context.Context, l *slog.Logger, m *menu.Menu[string], c *ui.Console) ([]*bookmark.Bookmark, error) {
	log := l.With("cmp", "gecko", "fn", "import")

	profilesPath, bookmarksPath := b.processPaths()

	if profilesPath == "" {
		log.Error("profiles path is empty")
		return nil, fmt.Errorf("%w: profiles filepath: empty", ErrBrowserConfigPathNotSet)
	}

	if bookmarksPath == "" {
		log.Error("bookmarks path is empty")
		return nil, fmt.Errorf("%w: bookmarks filepath: empty", ErrBrowserConfigPathNotSet)
	}

	if !files.Exists(profilesPath) {
		log.Warn("browser profiles configuration not found")
		return nil, fmt.Errorf("%w: %q", os.ErrNotExist, profilesPath)
	}

	profiles, err := allProfiles(profilesPath, l)
	if err != nil {
		return nil, err
	}

	p := c.Palette()
	c.Frame().
		Header(p.Yellow.Wrap(b.Name(), p.Bold)).
		Textln(p.Dim.Wrap(fmt.Sprintf(" (%d profiles found)", len(profiles)), p.Italic)).
		Rowln().
		Flush()

	n := len(profiles)
	log.Debug("browser profiles discovered", "count", n)

	if n == 0 {
		log.Debug("no browser profiles found")
		return nil, nil
	}

	if n > 1 {
		profiles, err = profileSelection(ctx, log, m, c, profiles)
		if err != nil {
			return nil, err
		}
	}

	var bs []*bookmark.Bookmark
	processed, skipped := 0, 0

	profileLog := l.With("cmp", "gecko", "stage", "process-profile")

	for name, hash := range profiles {
		if err := ctx.Err(); err != nil {
			log.Debug("profile import canceled",
				"processed", processed,
				"skipped", skipped,
				"error", err,
			)
			return nil, err
		}

		profile := browser.NewProfile(name, fmt.Sprintf(bookmarksPath, hash), b.Name(), b.Engine())
		if err := processProfile(ctx, profileLog, c, profile); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Error("failed to process browser profile",
					"profile", name,
					"error", err,
				)
				return nil, err
			}

			skipped++
			log.Debug("skipping profile",
				"profile", name,
				"reason", "bookmarks database not found",
			)

			c.Term().ClearLine(1)

			pf := p.Italic.Wrap(profile.Name+":", p.Bold)
			skip := p.BrightYellow.Wrap("skipping")
			reason := p.Italic.Sprint("no bookmarks found")

			c.Warning(fmt.Sprintf("%s profile %s %s", skip, pf, reason)).
				Ln().
				Flush()

			continue
		}

		fresh, duplicates := bookmark.DeduplicateURL(bs, profile.Bookmarks)

		bs = append(bs, fresh...)

		if len(duplicates) > 0 {
			log.Debug("duplicate bookmarks skipped",
				"profile", name,
				"count", len(duplicates),
			)
		}
		processed++

		log.Debug("profile processed",
			"profile", name,
			"bookmarks", len(profile.Bookmarks),
		)
	}

	log.Debug("browser import results",
		"profiles_processed", processed,
		"profiles_skipped", skipped,
		"bookmarks", len(bs),
	)

	return bs, nil
}

func (b *GeckoBrowser) processPaths() (profilePath, bookmarksPath string) {
	for i := range b.paths.profiles {
		if files.Exists(b.paths.profiles[i]) {
			profilePath = b.paths.profiles[i]
			bookmarksPath = b.paths.bookmarks[i]
			break
		}
	}

	return profilePath, bookmarksPath
}

type geckoBookmark struct {
	FK     int    `db:"fk"`
	Parent int    `db:"parent"`
	Title  string `db:"title"`
	URL    string `db:"url"`
	Tags   string `db:"tags"`
}

// openSQLite opens the SQLite database and returns a *sql.DB object.
func openSQLite(ctx context.Context, c *ui.Console, profileName, dbPath string) (*sqlx.DB, error) {
	cfg, err := db.NewSQLiteCfg(fmt.Sprintf("file:%s?cache=shared", dbPath))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()

	p := c.Palette()

	s := rotato.New(
		rotato.WithPrefix(profileName),
		rotato.WithPrefixColor(p.Gray.Format),
		rotato.WithMessage("connecting to database..."),
		rotato.WithMessageColor(p.BrightBlue.Format),
		rotato.WithSpinnerColor(p.Gray.Format),
		rotato.WithFailMessageColor(p.BrightRed.Format),
		rotato.WithFailSymbolColor(p.BrightRed.Format),
		rotato.WithPrefixDecorator(func(mesg string) string {
			remaining := max(time.Until(deadline).Round(time.Second), 0)
			return mesg + " " + p.Gray.Sprint(rotato.CountdownDecorator(remaining))
		}),
	)
	s.Start(ctx)
	defer s.Done()

	r, err := db.Connect(ctx, dbPath, cfg)
	if err != nil {
		s.Fail("connecting to database failed")
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
func queryBookmarks(r *sqlx.DB, profileSource, profileName string) ([]*geckoBookmark, error) {
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

		tags, err := processTags(r, gb.FK)
		if err != nil {
			return err
		}

		gb.Tags = bookmark.ParseTags(profileSource + "-" + profileName + "," + tags)

		return nil
	}

	for _, b := range bs {
		if err := parseGeckoBookmark(b); err != nil {
			return nil, err
		}
	}

	return bs, nil
}

func allProfiles(p string, l *slog.Logger) (map[string]string, error) {
	log := l.With("cmp", "gecko", "fn", "all-profiles")

	log.Debug("reading browser profiles configuration")

	inidata, err := ini.Load(p)
	if err != nil {
		log.Error("failed to load profiles configuration", "error", err)
		return nil, fmt.Errorf("load profiles configuration: %w", err)
	}

	result := make(map[string]string)

	for _, sec := range inidata.Sections() {
		if !strings.HasPrefix(sec.Name(), "Profile") {
			continue
		}

		name := sec.Key("Name").String()
		path := sec.Key("Path").String()

		if name == "" || path == "" {
			log.Warn("skipping incomplete profile entry",
				"section", sec.Name(),
				"has_name", name != "",
				"has_path", path != "",
			)
			continue
		}

		result[name] = path
	}

	log.Debug("browser profiles loaded", "count", len(result))

	return result, nil
}

// processProfile processes a single profile and extracts bookmarks.
func processProfile(ctx context.Context, l *slog.Logger, c *ui.Console, profile *browser.Profile) error {
	log := l.With("profile", profile.Name)

	log.Debug("processing browser profile")

	path := files.ExpandHomeDir(profile.Path)
	if err := files.ExistsErr(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Debug("profile database not found")
		} else {
			log.Error("profile database path check failed", "error", err)
		}

		return fmt.Errorf("%w: %q", err, path)
	}

	log.Debug("opening profile database")

	r, err := openSQLite(ctx, c, profile.Name, path)
	if err != nil {
		log.Error("failed to open profile database", "error", err)
		handleDBError(c, profile.Name, err)
		// return err
		return nil
	}

	defer func() {
		if r == nil {
			return
		}

		if err = r.Close(); err != nil {
			log.Warn("failed to close profile database", "error", err)
			return
		}

		log.Debug("profile database closed")
	}()

	gmarks, err := queryBookmarks(r, strings.ToLower(profile.Source), profile.Name)
	if err != nil {
		log.Error("failed to query profile bookmarks", "error", err)
		fmt.Fprintf(c.Writer(), "error querying bookmarks for profile %q: %v\n", profile.Name, err)
		return err
	}

	log.Debug("profile bookmarks queried", "count", len(gmarks))

	skipped := importBookmarks(profile, gmarks, c.Palette())
	imported := len(profile.Bookmarks) - skipped

	log.Debug("profile bookmarks extracted",
		"imported", imported,
		"skipped", skipped,
	)

	found := c.Palette().BrightBlue.Sprint("found")
	c.Info(fmt.Sprintf("%s %d bookmarks in profile %q\n", found, imported, profile.Name)).Flush()

	return nil
}

func handleDBError(c *ui.Console, profile string, err error) {
	p := c.Palette()
	slog.Error("opening database for profile", "profile", profile, "err", err)
	if errors.Is(err, ErrBrowserIsOpen) {
		c.Error("profile " + profile + ": database is " + p.BrightRed.Sprint("locked") + ", maybe browser is open?\n").Flush()
		return
	}
	fmt.Fprintf(c.Writer(), "err opening database for profile %q: %v\n", profile, err)
}

func importBookmarks(profile *browser.Profile, gmarks []*geckoBookmark, p *ansi.Palette) int {
	skipped := 0
	profileColor := p.Random()

	for _, gb := range gmarks {
		if gb.URL == "" {
			continue
		}
		b := bookmark.New()
		b.Title = gb.Title
		b.URL = gb.URL
		b.Tags = gb.Tags
		b.IsActive = true
		b.Meta.Origin = profileColor.Sprint(profile.Name)
		b.Meta.Source = p.Red.Sprint(strings.ToLower(profile.Source))

		if isDuplicate(profile.Bookmarks, b.URL) {
			skipped++
			continue
		}
		profile.Bookmarks = append(profile.Bookmarks, b)
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

func profileSelection(ctx context.Context, log *slog.Logger, m *menu.Menu[string], c *ui.Console, profiles map[string]string) (map[string]string, error) {
	n := len(profiles)

	if n == 0 {
		log.Debug("no browser profiles available")
		return profiles, nil
	}

	if n == 1 {
		log.Debug("automatically selecting the only profile")
		return profiles, nil
	}

	log.Debug("requesting profile selection", "count", n)

	opt, err := c.Choose(ctx, "process all profiles?", []string{"yes", "select"}, "yes")
	if err != nil {
		log.Debug("profile selection failed", "error", err)
		return nil, err
	}

	switch opt {
	case "yes":
		log.Debug("all profiles selected", "count", n)
		return profiles, nil

	case "select":
		names := slices.Sorted(maps.Keys(profiles))

		selected, err := m.Select(ctx, names)
		if err != nil {
			log.Debug("profile selection failed", "error", err)
			return nil, err
		}

		result := make(map[string]string, len(selected))
		for _, name := range selected {
			if path, ok := profiles[name]; ok {
				result[name] = path
			}
		}

		log.Debug("profiles selected", "count", len(result))

		return result, nil

	default: // cancel
		log.Debug("profile import canceled")
		return nil, application.ErrActionAborted
	}
}
