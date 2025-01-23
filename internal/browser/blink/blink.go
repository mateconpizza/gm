package blink

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/haaag/gm/internal/bookmark"
	browserpath "github.com/haaag/gm/internal/browser/paths"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

type Record = bookmark.Bookmark

var (
	ErrBrowserConfigPathNotSet = errors.New("browser config path not set")
	ErrBrowserUnsupported      = errors.New("browser is unsupported")
)

var blinkBrowserPaths = map[string]Paths{
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

type Paths struct {
	profiles  string
	bookmarks string
}

type BlinkBrowser struct {
	name  string
	short string
	color color.ColorFn
	paths Paths
}

func (b *BlinkBrowser) Name() string {
	return b.name
}

func (b *BlinkBrowser) Short() string {
	return b.short
}

func (b *BlinkBrowser) Color(s string) string {
	return b.color(s).Bold().String()
}

func (b *BlinkBrowser) LoadPaths() error {
	p, ok := blinkBrowserPaths[b.name]
	if !ok {
		return fmt.Errorf("%w: '%s'", ErrBrowserUnsupported, b.name)
	}
	b.paths = p

	return nil
}

// Import extracts profile system names and user names.
func (b *BlinkBrowser) Import(t *terminal.Term) (*slice.Slice[bookmark.Bookmark], error) {
	p := b.paths
	if p.bookmarks == "" || p.profiles == "" {
		return nil, ErrBrowserConfigPathNotSet
	}
	if !files.Exists(p.profiles) {
		return nil, fmt.Errorf("%w: '%s'", files.ErrFileNotFound, p.profiles)
	}

	jsonData, err := os.ReadFile(p.profiles)
	if err != nil {
		return nil, fmt.Errorf("error reading JSON file: %w", err)
	}

	profiles, err := processChromiumProfiles(jsonData)
	if err != nil {
		return nil, err
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Header(fmt.Sprintf("Starting %s import...", b.Color(b.Name()))).Ln()
	f.Mid(fmt.Sprintf("Found %d profiles", len(profiles))).Ln().Render()

	bs := slice.New[Record]()
	for profile, v := range profiles {
		p := fmt.Sprintf(p.bookmarks, profile)
		processProfile(t, bs, v, files.ExpandHomeDir(p))
	}

	return bs, nil
}

func New(name string, c color.ColorFn) *BlinkBrowser {
	return &BlinkBrowser{
		name:  name,
		short: strings.ToLower(string(name[0])),
		color: c,
	}
}

// structure of the JSON bookmarks file.
type JSONRoot struct {
	Roots map[string]interface{} `json:"roots"`
}

// structure of the JSON profile file.
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
func traverseBmFolder(
	children []interface{},
	uniqueTag string,
	parentName string,
	addParentFolderAsTag bool,
) [][]string {
	var results [][]string
	for _, child := range children {
		childMap, ok := child.(map[string]interface{})
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
		childrenVal, ok := childMap["children"].([]interface{})
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
		result[systemName] = info.Name
	}

	return result, nil
}

// processProfile extracts profile system names and user names.
func processProfile(t *terminal.Term, bs *slice.Slice[Record], profile, path string) {
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())

	if !files.Exists(path) {
		s := "Skipping profile...'" + profile + "', bookmarks file not found"
		f.Row().Ln().Header(s).Ln().Render()
		return
	}

	f.Row().Ln().Render().Clean()
	f.Header(fmt.Sprintf("import bookmarks from '%s' profile?", profile))
	if !t.Confirm(f.String(), "n") {
		t.ClearLine(1)
		f.Clean().Row("Skipping profile...'" + profile + "'").Ln().Render()
		return
	}

	uniqueTag := getTodayFormatted()
	addParentFolderAsTag := true
	result, err := loadChromeDatabase(path, uniqueTag, addParentFolderAsTag)
	if err != nil {
		fmt.Println("Error loading Chrome database:", err)
	}

	// original size
	ogSize := bs.Len()
	for _, c := range result {
		b := bookmark.New()
		b.Title = c.title
		b.URL = c.url
		b.Tags = bookmark.ParseTags(strings.Join(c.tags, ","))

		if bs.Has(func(b bookmark.Bookmark) bool {
			return b.URL == c.url
		}) {
			continue
		}

		bs.Append(b)
	}

	found := color.BrightBlue("found")
	f.Clean().Info(fmt.Sprintf("%s %d bookmarks", found, bs.Len()-ogSize)).Ln().Render()
}

// Define the main function to load the Chrome database.
func loadChromeDatabase(
	path, uniqueTag string,
	addParentFolderAsTag bool,
) ([]blinkBookmark, error) {
	byteValue, _ := os.ReadFile(path)

	s := spinner.New(
		spinner.WithMesg(color.BrightBlue("parsing bookmark file...").String()),
		spinner.WithColor(color.Gray),
	)
	s.Start()
	defer s.Stop()

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
		valueMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		children, ok := valueMap["children"].([]interface{})
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

func getTodayFormatted() string {
	today := time.Now()
	return today.Format("2006Jan02")
}
