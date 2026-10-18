package bookio

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

const (
	DefaultGlyphs  = "emoji"
	DefaultSummary = "badges"
	DefaultType    = "default"

	ReadmeFile = "README.md"
)

var (
	SupportedGlyphs  = []string{"ascii", "emoji", "none"}
	SupportedTypes   = []string{"default", "domain", "tag"}
	SupportedSummary = []string{"badges", "minimal", "table", "none"}
)

type (
	Decorator    func(sb *strings.Builder, b *bookmark.Bookmark)
	Preprocessor func(bs []*bookmark.Bookmark) []*bookmark.Bookmark
	Sorter       func(bs []*bookmark.Bookmark)

	MdOptFunc func(*Markdown)
)

type MarkdownSummary struct {
	Bookmarks   int
	Tags        int
	Favorites   int
	Archived    int
	DeadLinks   int
	TotalVisits int
}

type Markdown struct {
	title         string
	bookmarks     []*bookmark.Bookmark
	decorators    []Decorator
	preprocessors []Preprocessor
	sorter        Sorter
	summary       func() string

	writer io.Writer
}

func NewMarkdown(bs []*bookmark.Bookmark, opts ...MdOptFunc) *Markdown {
	md := &Markdown{bookmarks: bs, writer: os.Stdout}
	for _, opt := range opts {
		opt(md)
	}

	if md.sorter == nil {
		md.sorter = defaultSorter
	}
	return md
}

func WithTitle(s string) MdOptFunc           { return func(md *Markdown) { md.title = s } }
func WithSorter(fn Sorter) MdOptFunc         { return func(md *Markdown) { md.sorter = fn } }
func WithWriter(w io.Writer) MdOptFunc       { return func(md *Markdown) { md.writer = w } }
func WithSummary(fn func() string) MdOptFunc { return func(md *Markdown) { md.summary = fn } }
func WithDecorator(fn Decorator) MdOptFunc {
	return func(md *Markdown) { md.decorators = append(md.decorators, fn) }
}

func WithPreprocessors(fn Preprocessor) MdOptFunc {
	return func(md *Markdown) { md.preprocessors = append(md.preprocessors, fn) }
}

func (md *Markdown) Render() error {
	_, err := fmt.Fprint(md.writer, md.build())
	return err
}

func (md *Markdown) build() string {
	bs := md.bookmarks
	for _, fn := range md.preprocessors {
		md.bookmarks = fn(md.bookmarks)
	}

	md.sorter(bs)

	var sb strings.Builder
	title := md.title
	if title == "" {
		title = "Bookmarks"
	}
	fmt.Fprintf(&sb, "# %s\n\n", title)

	if md.summary != nil && md.summary() != "" {
		fmt.Fprintf(&sb, "%s\n>\n", md.summary())
	}

	fmt.Fprintf(&sb, "> updated %s\n\n", time.Now().Format("2006-01-02"))

	for _, b := range bs {
		writeLine(&sb, b, md.decorators...)
	}

	return sb.String()
}

func MarkdownSummaryFmt(t string, s *MarkdownSummary) func() string {
	return func() string {
		switch t {
		case "badges":
			return summaryBadges(s)
		case "table":
			return summaryTable(s)
		case "minimal":
			return summaryMinimal(s)
		default:
			return ""
		}
	}
}

func MarkdownGlyphsDecorator(s string) func(sb *strings.Builder, b *bookmark.Bookmark) {
	emoji := &markdownGlyphs{
		Favorite: "\u2B50",
		Notes:    "\U0001F4DD",
		Archive:  "\U0001F4E6",
		Broken:   "\U0001F494",
		Inactive: "\u23F8\uFE0F",
	}

	switch s {
	case "ascii":
		return glyphsDecorator(&markdownGlyphs{
			Favorite: "Favorite",
			Notes:    "Notes",
			Archive:  "Archive",
			Broken:   "Broken",
			Inactive: "Inactive",
		})

	case "emoji":
		return glyphsDecorator(emoji)

	case "none":
		return glyphsDecorator(&markdownGlyphs{Archive: "@"})

	default:
		return glyphsDecorator(emoji)
	}
}

func summaryBadges(s *MarkdownSummary) string {
	badge := func(label string, value int, color string) string {
		return fmt.Sprintf(
			"![%s](https://img.shields.io/badge/%s-%d-%s)",
			label, strings.ReplaceAll(strings.ToLower(label), " ", "_"), value, color,
		)
	}

	badges := make([]string, 0, 6)
	badges = append(badges,
		badge("Bookmarks", s.Bookmarks, "blue"),
		badge("Tags", s.Tags, "informational"),
	)

	if s.Favorites > 0 {
		badges = append(badges,
			badge("Favorites", s.Favorites, "yellow"),
		)
	}
	if s.Archived > 0 {
		badges = append(badges,
			badge("Snapshot", s.Archived, "important"),
		)
	}
	if s.DeadLinks > 0 {
		badges = append(badges,
			badge("Dead Links", s.DeadLinks, "critical"),
		)
	}

	badges = append(badges, badge("Total Visits", s.TotalVisits, "success"))
	return strings.Join(badges, " ")
}

func summaryMinimal(s *MarkdownSummary) string {
	return fmt.Sprintf(
		"> %d bookmarks · %d tags · %d favorites · %d dead links",
		s.Bookmarks, s.Tags, s.Favorites, s.DeadLinks,
	)
}

func summaryTable(s *MarkdownSummary) string {
	var sb strings.Builder

	sb.WriteString("| Bookmarks | Tags | Favorites | Snapshot | Dead Links | Total Visits |\n")
	sb.WriteString("| --------- | ---- | --------- | -------- | ---------- | ------------ |\n")

	fmt.Fprintf(&sb, "| %d | %d | %d | %d | %d | %d |\n",
		s.Bookmarks, s.Tags, s.Favorites, s.Archived, s.DeadLinks, s.TotalVisits)

	return sb.String()
}

func writeLine(sb *strings.Builder, b *bookmark.Bookmark, decorators ...Decorator) {
	title := b.Title
	if title == "" {
		title = b.URL
	}

	fmt.Fprintf(sb, "- [%s](%s)", markdownEscape(title), b.URL)

	if b.Desc != "" {
		fmt.Fprintf(sb, " - %s", markdownEscape(b.Desc))
	}

	for t := range strings.SplitSeq(b.Tags, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			fmt.Fprintf(sb, " `%s`", t)
		}
	}

	for _, d := range decorators {
		d(sb, b)
	}

	sb.WriteByte('\n')
}

// markdownEscape escapes characters that would break Markdown link/text syntax.
func markdownEscape(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		"[", "\\[",
		"]", "\\]",
		"|", "\\|",
		"#", "\\#",
		"*", "\\*",
		"_", "\\_",
		"`", "\\`",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

type markdownGlyphs struct {
	Favorite string
	Notes    string
	Archive  string
	Broken   string
	Inactive string
}

func glyphsDecorator(g *markdownGlyphs) func(sb *strings.Builder, b *bookmark.Bookmark) {
	return func(sb *strings.Builder, b *bookmark.Bookmark) {
		if b.Favorite {
			sb.WriteByte(' ')
			sb.WriteString(g.Favorite)
		}
		if b.Notes != "" {
			sb.WriteByte(' ')
			sb.WriteString(g.Notes)
		}
		if b.ArchiveURL != "" {
			sb.WriteByte(' ')
			fmt.Fprintf(sb, "[%s](%s %q)", g.Archive, b.ArchiveURL, "Wayback Machine Snapshot")
		}
		if b.HTTPStatusCode >= 400 {
			sb.WriteByte(' ')
			sb.WriteString(g.Broken)
		}
		if !b.IsActive {
			sb.WriteByte(' ')
			sb.WriteString(g.Inactive)
		}
	}
}

func defaultSorter(bs []*bookmark.Bookmark) {
	sort.Slice(bs, func(i, j int) bool {
		return bs[i].ID < bs[j].ID
	})
}
