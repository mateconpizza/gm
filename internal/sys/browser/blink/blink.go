// Package blink provides functionalities for importing bookmarks from
// Blink-based web browsers like Chromium, Chrome, and Edge.
package blink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	menu "github.com/mateconpizza/go-fzf"
	files "github.com/mateconpizza/gofiles"
	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/sys/browser"
	browserpath "github.com/mateconpizza/gm/internal/sys/browser/paths"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrBrowserConfigPathNotSet = errors.New("browser config path not set")

const Blink browser.Engine = "blink"

var _ browser.Browser = (*BlinkBrowser)(nil)

var Supported = []browser.Supported{
	{Browser: New("Chromium")},
	{Browser: New("Google Chrome")},
	{Browser: New("Brave")},
	{Browser: New("Vivaldi")},
	{Browser: New("Edge")},
}

var browserPaths = map[string]Paths{
	"Chromium": {
		profiles:  browserpath.BlinkProfilePath("chromium"),
		bookmarks: browserpath.BlinkBookmarksPath("chromium"),
	},
	"Google Chrome": {
		profiles:  browserpath.BlinkProfilePath("google-chrome"),
		bookmarks: browserpath.BlinkBookmarksPath("google-chrome"),
	},
	"Edge": {
		profiles:  browserpath.BlinkProfilePath("microsoft-edge"),
		bookmarks: browserpath.BlinkBookmarksPath("microsoft-edge"),
	},
	"Brave": {
		profiles:  browserpath.BlinkProfilePath("brave"),
		bookmarks: browserpath.BlinkBookmarksPath("brave"),
	},
	"Vivaldi": {
		profiles:  browserpath.BlinkProfilePath("vivaldi"),
		bookmarks: browserpath.BlinkBookmarksPath("vivaldi"),
	},
}

func Register(name, path string) *BlinkBrowser {
	name = txt.Capitalize(name)
	p := Paths{
		profiles:  browserpath.BlinkProfilePath(path),
		bookmarks: browserpath.BlinkBookmarksPath(path),
	}
	browserPaths[name] = p
	return &BlinkBrowser{
		name:  name,
		paths: p,
	}
}

type Paths struct {
	profiles  string
	bookmarks string
}

type BlinkBrowser struct {
	name  string
	paths Paths
}

func (b *BlinkBrowser) Name() string           { return b.name }
func (b *BlinkBrowser) String() string         { return ansi.BrightBlue.Sprint(b.name) }
func (b *BlinkBrowser) Engine() browser.Engine { return Blink }

func (b *BlinkBrowser) LoadPaths(l *slog.Logger) error {
	log := l.With("cmp", "blink", "fn", "load-paths", "browser", b.name)
	log.Debug("loading browser paths")

	p, ok := browserPaths[b.name]
	if !ok {
		err := fmt.Errorf("%w: %q", browser.ErrUnsupported, b.name)
		log.Warn("unsupported browser", "error", err)
		return err
	}

	b.paths = p

	log.Debug("browser paths loaded")

	return nil
}

// Import extracts profile system names and user names.
func (b *BlinkBrowser) Import(ctx context.Context, l *slog.Logger, m *menu.Menu[string], c *ui.Console) ([]*bookmark.Bookmark, error) {
	log := l.With("cmp", "blink", "fn", "import")
	if b.paths.bookmarks == "" {
		log.Error("bookmarks path is empty")
		return nil, ErrBrowserConfigPathNotSet
	}

	if b.paths.profiles == "" {
		log.Error("profiles path is empty")
		return nil, ErrBrowserConfigPathNotSet
	}

	if !files.Exists(b.paths.profiles) {
		log.Warn("browser profiles configuration not found")
		return nil, fmt.Errorf("%w: %q", os.ErrNotExist, b.paths.profiles)
	}

	jsonData, err := os.ReadFile(b.paths.profiles)
	if err != nil {
		return nil, fmt.Errorf("error reading JSON file: %w", err)
	}

	profiles, err := processChromiumProfiles(jsonData)
	if err != nil {
		return nil, err
	}

	p := c.Palette()
	c.Frame().
		Rowln().
		Header(p.BrightBlue.Wrap(b.Name(), p.Bold)).
		Textln(p.Dim.Wrap(fmt.Sprintf(" (%d profiles found)", len(profiles)), p.Italic)).
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

	profileLog := l.With("cmp", "blink", "stage", "process-profile")

	for name, systemName := range profiles {
		if err := ctx.Err(); err != nil {
			log.Debug("profile import canceled",
				"processed", processed,
				"skipped", skipped,
				"error", err,
			)
			return nil, err
		}

		bookmarksPath := fmt.Sprintf(b.paths.bookmarks, systemName)
		profile := browser.NewProfile(name, files.ExpandHomeDir(bookmarksPath), b.Name(), b.Engine())
		if err := processProfile(ctx, profileLog, c, profile); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Error("failed to process browser profile",
					"profile", systemName,
					"error", err,
				)
				return nil, err
			}

			skipped++
			log.Debug("skipping profile",
				"profile", systemName,
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
				"profile", systemName,
				"count", len(duplicates),
			)
		}
		processed++

		log.Debug("profile processed",
			"profile", systemName,
			"bookmarks", len(profile.Bookmarks),
		)
	}

	return bs, nil
}

func New(name string) *BlinkBrowser {
	return &BlinkBrowser{name: name}
}

// JSONRoot structure of the JSON bookmarks file.
type JSONRoot struct {
	Roots map[string]any `json:"roots"`
}

// JSONProfile structure of the JSON profile file.
//
//	"profile": {
//	    "info_cache": {
//	        "Profile 1": {...},
//	        "Profile 2": {...},
//	        ...
//	    }
//	}
type JSONProfile struct {
	InfoCache map[string]struct {
		Name string `json:"name"`
	} `json:"info_cache"`
}

type JSONData struct {
	Profile JSONProfile `json:"profile"`
}

type blinkBookmark struct {
	title string
	url   string
	tags  []string
}

// Define a function to traverse the bookmark folder.
func traverseBmFolder(children []any, uniqueTag, parentName string, addParentFolderAsTag bool) [][]string {
	var results [][]string

	for _, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		// Get the name and URL of the bookmark
		name, ok := childMap["name"].(string)
		if !ok {
			name = ""
		}

		url, ok := childMap["url"].(string)
		if !ok {
			url = ""
		}

		// Check if the bookmark is a folder
		typeStr, ok := childMap["type"].(string)
		if !ok || typeStr != "folder" {
			tags := []string{uniqueTag}
			if addParentFolderAsTag {
				tags = append(tags, parentName)
			}

			item := append([]string{name, url}, tags...)
			results = append(results, item)

			continue
		}

		// Recursively traverse the folder
		childrenVal, ok := childMap["children"].([]any)
		if !ok {
			continue
		}

		// Add the unique tag to the folder
		results = append(
			results,
			traverseBmFolder(
				childrenVal,
				uniqueTag,
				name,
				addParentFolderAsTag,
			)...,
		)
	}

	return results
}

// Function to extract profile system names and user names.
func processChromiumProfiles(jsonData []byte) (map[string]string, error) {
	var data JSONData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	result := make(map[string]string)
	for systemName, info := range data.Profile.InfoCache {
		result[info.Name] = systemName
	}

	return result, nil
}

// processProfile extracts profile system names and user names.
func processProfile(ctx context.Context, l *slog.Logger, c *ui.Console, profile *browser.Profile) error {
	log := l.With("fn", "process-profile", "profile", profile.Name)

	if err := ctx.Err(); err != nil {
		log.Debug("profile processing canceled", "error", err)
		return err
	}

	f, p := c.Frame(), c.Palette()
	skip := p.BrightYellow.Sprint("skipping")
	pf := p.Italic.Wrap(profile.Name, p.Bold)

	if !files.Exists(profile.Path) {
		log.Warn("profile bookmarks file not found, skipping",
			"path", filepath.Base(profile.Path),
		)

		f.Reset()
		f.Warning(skip + " profile " + pf + p.Italic.Sprint(": bookmarks file not found\n")).Flush()
		return nil
	}

	log.Debug("loading Chrome bookmarks database")

	uniqueTag := strings.ToLower(profile.Source) + "-" + profile.Name
	addParentFolderAsTag := true

	result, err := loadChromeDatabase(
		ctx,
		profile.Path,
		uniqueTag,
		addParentFolderAsTag,
		c.Palette().Enabled(),
	)
	if err != nil {
		log.Error("failed to load Chrome bookmarks database", "error", err)
		fmt.Fprintln(c.Writer(), "Error loading Chrome database:", err)
	}

	log.Debug("Chrome bookmarks loaded", "count", len(result))

	seen := make(map[string]struct{}, len(profile.Bookmarks)+len(result))
	for _, existing := range profile.Bookmarks {
		if existing != nil {
			seen[existing.URL] = struct{}{}
		}
	}

	imported, duplicates := 0, 0

	profileColor := p.Random()

	for _, item := range result {
		if _, exists := seen[item.url]; exists {
			duplicates++
			continue
		}

		b := bookmark.New()
		b.Title = item.title
		b.URL = item.url
		b.Tags = bookmark.ParseTags(strings.Join(item.tags, ","))
		b.Meta.Origin = profileColor.Sprint(profile.Name)
		b.Meta.Source = p.Red.Sprint(strings.ToLower(profile.Source))

		seen[b.URL] = struct{}{}
		profile.Bookmarks = append(profile.Bookmarks, b)
		imported++
	}

	log.Debug("profile bookmarks processed",
		"imported", imported,
		"duplicates", duplicates,
		"total", len(profile.Bookmarks),
	)

	found := p.BrightBlue.Sprint("found")
	c.Info(fmt.Sprintf("%s %d bookmarks in profile %q\n", found, imported, profile.Name)).Flush()

	return nil
}

// Define the main function to load the Chrome database.
func loadChromeDatabase(ctx context.Context, path, uniqueTag string, addParentFolderAsTag, color bool) ([]blinkBookmark, error) {
	byteValue, _ := os.ReadFile(path)
	p := ansi.NewPalette(color)

	s := rotato.New(
		rotato.WithMessage("parsing bookmark file..."),
		rotato.WithMessageColor(p.BrightBlue.Format),
		rotato.WithSpinnerColor(p.Gray.Format),
	)
	s.Start(ctx)
	defer s.Done()

	// unmarshal the json data
	var data JSONRoot
	if err := json.Unmarshal(byteValue, &data); err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	// traverse the roots
	results := make([]blinkBookmark, 0)
	roots := data.Roots

	for _, value := range roots {
		if _, ok := value.(string); ok {
			continue
		}

		valueMap, ok := value.(map[string]any)
		if !ok {
			continue
		}

		children, ok := valueMap["children"].([]any)
		if !ok {
			continue
		}

		parentName, ok := valueMap["name"].(string)
		if !ok {
			continue
		}

		for _, item := range traverseBmFolder(children, uniqueTag, parentName, addParentFolderAsTag) {
			c := &blinkBookmark{
				title: item[0],
				url:   item[1],
				tags:  item[2:],
			}
			results = append(results, *c)
		}
	}

	return results, nil
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
