package gecko

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	ini "gopkg.in/ini.v1"

	"github.com/haaag/gm/internal/bookmark"
	browserpath "github.com/haaag/gm/internal/browser/paths"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

var (
	ErrBrowserIsOpen           = errors.New("browser is open")
	ErrBrowserConfigPathNotSet = errors.New("browser config path not set")
	ErrBrowserUnsupported      = errors.New("browser is unsupported")
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
	color color.ColorFn
	paths Paths
}

func (b *GeckoBrowser) Name() string {
	return b.name
}

func (b *GeckoBrowser) Short() string {
	return b.short
}

func (b *GeckoBrowser) Color(s string) string {
	return b.color(s).Bold().String()
}

func (b *GeckoBrowser) LoadPaths() error {
	p, ok := geckoBrowserPaths[b.name]
	if !ok {
		return fmt.Errorf("%w: '%s'", ErrBrowserUnsupported, b.name)
	}
	b.paths = p

	return nil
}

func (b *GeckoBrowser) Import() (*slice.Slice[bookmark.Bookmark], error) {
	p := b.paths
	if p.profiles == "" || p.bookmarks == "" {
		return nil, ErrBrowserConfigPathNotSet
	}
	if !files.Exists(p.profiles) {
		return nil, fmt.Errorf("%w: '%s'", files.ErrFileNotFound, p.profiles)
	}

	profiles, err := allProfiles(p.profiles)
	if err != nil {
		return nil, err
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Header(fmt.Sprintf("Starting %s import...", b.Color(b.Name()))).Ln()
	f.Mid(fmt.Sprintf("Found %d profiles!", len(profiles))).Ln().Render()

	bs := slice.New[bookmark.Bookmark]()
	for profile, v := range profiles {
		p := fmt.Sprintf(p.bookmarks, v)
		processProfile(bs, profile, p)
	}

	return bs, nil
}

func New(name string, c color.ColorFn) *GeckoBrowser {
	return &GeckoBrowser{
		name:  name,
		short: strings.ToLower(string(name[0])),
		color: c,
	}
}

type geckoBookmark struct {
	fk     int
	parent int
	title  string
	url    string
	tags   string
}

// openSQLite opens the SQLite database and returns a *sql.DB object.
func openSQLite(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := spinner.New(
		spinner.WithMesg(color.BrightBlue("connecting to database...").String()),
		spinner.WithColor(color.Gray),
	)
	s.Start()
	defer s.Stop()

	// Check if the database is reachable
	if err = db.Ping(); err != nil {
		log.Printf("failed to ping database: %v\n", err)
		if err.Error() == "database is locked" {
			return nil, ErrBrowserIsOpen
		}

		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// isNonGenericURL checks if the given URL is a non-generic URL based on a set
// of ignored prefixes.
func isNonGenericURL(url string) bool {
	log.Println("isNonGenericURL checking url:", url)
	ignoredPrefixes := []string{
		"about:",
		"apt:",
		"chrome://",
		"file://",
		"place:",
		"vivaldi://",
		"moz-extension://",
	}

	for _, prefix := range ignoredPrefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}

	return false
}

// queryBookmarks queries the bookmarks table to retrieve some sample data.
func queryBookmarks(db *sql.DB) ([]geckoBookmark, error) {
	rows, err := db.Query(
		"SELECT DISTINCT fk, parent, title FROM moz_bookmarks WHERE type=1 AND title IS NOT NULL",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query bookmarks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	var gb geckoBookmark
	var bookmarks []geckoBookmark
	for rows.Next() {
		var title, id, parent string
		if err := rows.Scan(&gb.fk, &gb.parent, &gb.title); err != nil {
			fmt.Printf(
				"failed to scan title, id row: title=%v id=%v parent=%v\n",
				title,
				id,
				parent,
			)

			continue
		}

		gb.url, err = processURLs(db, gb.fk)
		if err != nil {
			return nil, err
		}

		if gb.url == "" {
			continue
		}

		gb.tags, err = processTags(db, gb.fk)
		if err != nil {
			return nil, err
		}

		gb.tags += " " + getTodayFormatted()
		gb.tags = bookmark.ParseTags(gb.tags)
		bookmarks = append(bookmarks, gb)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return bookmarks, nil
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

func processProfile(bs *slice.Slice[bookmark.Bookmark], profileName, dbPath string) {
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Row().Ln().Render()
	f.Clean().Header(fmt.Sprintf("import bookmarks from '%s' profile?", profileName))
	if !terminal.Confirm(f.String(), "n") {
		return
	}

	// original size
	ogSize := bs.Len()

	// FIX: get path by OS
	dbPath = files.ExpandHomeDir(dbPath)
	db, err := openSQLite(dbPath)
	if err != nil {
		log.Printf("err opening database for profile '%s': %v\n", profileName, err)
		if errors.Is(err, ErrBrowserIsOpen) {
			l := color.BrightRed("locked").String()
			f.Clean().Error("database is " + l + ", maybe firefox is open?").Ln().Render()
			return
		}
		fmt.Printf("err opening database for profile '%s': %v\n", profileName, err)

		return
	}

	gmarks, err := queryBookmarks(db)
	if err != nil {
		fmt.Printf("Error querying bookmarks for profile '%s': %v\n", profileName, err)
		return
	}

	for _, m := range gmarks {
		if m.url == "" {
			continue
		}
		b := bookmark.New()
		b.Title = m.title
		b.URL = m.url
		b.Tags = m.tags

		if bs.Has(func(b bookmark.Bookmark) bool {
			return b.URL == m.url
		}) {
			continue
		}

		bs.Append(b)
	}

	if err := db.Close(); err != nil {
		log.Printf("err closing rows: %v", err)
	}

	found := color.BrightBlue("found")
	f.Clean().Info(fmt.Sprintf("%s %d bookmarks", found, bs.Len()-ogSize)).Ln().Render()
}

func processTags(db *sql.DB, fk int) (string, error) {
	var tags []string
	rows, err := db.Query("SELECT parent FROM moz_bookmarks WHERE fk=? AND title IS NULL", fk)
	if err != nil {
		return "", fmt.Errorf("failed to query tags: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	for rows.Next() {
		var tagID string
		if err := rows.Scan(&tagID); err != nil {
			return "", fmt.Errorf("failed to scan tag row: %w", err)
		}

		var tag string
		err := db.QueryRow("SELECT title FROM moz_bookmarks WHERE id=?", tagID).Scan(&tag)
		if err != nil {
			return "", fmt.Errorf("failed to query tag title: %w", err)
		}
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("rows iteration error: %w", err)
	}

	if len(tags) == 0 {
		return "", nil
	}

	return strings.Join(tags, ","), nil
}

func processURLs(db *sql.DB, fk int) (string, error) {
	var url string
	rows, err := db.Query("SELECT url FROM moz_places where id=?", fk)
	if err != nil {
		return url, fmt.Errorf("failed to query places: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
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
