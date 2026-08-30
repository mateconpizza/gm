package handler

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper"
)

type tagStore interface {
	TagsCounter(ctx context.Context) (map[string]int, error)
}

type storeReader interface {
	BaseName() string
	Name() string
	Stats(ctx context.Context, dest any) error
}

type bookmarkStore interface {
	storeReader

	InsertOne(ctx context.Context, b *bookmark.Bookmark) (int64, error)
	ByID(ctx context.Context, bID int) (*bookmark.Bookmark, error)
	All(ctx context.Context) ([]*bookmark.Bookmark, error)
}

type metadataScraper interface {
	Start(ctx context.Context) error

	Title() (string, error)
	Desc() (string, error)
	Favicon() (string, error)
	Keywords() (string, error)
	TagsRepo() ([]string, error)
}

type tagTerminal interface {
	ChooseTags(prompt string, items map[string]int) string
	ClearLine(n int)
}

type console interface {
	Frame() *frame.Frame
	Palette() *ansi.Palette
	Term() *terminal.Term

	SuccessMesg(a ...any) string
}

type bookmarkTemp struct {
	title, desc, tags, favicon string
}

func AddBookmark(ctx context.Context, d *deps.Deps, args []string) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()

	c, p := d.Console(), d.Console().Palette()
	title := p.BrightYellow.With(p.Bold).
		Sprint("Add Bookmark")
	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")
	name := p.BrightYellow.With(p.Bold).
		Sprint(r.BaseName())
	info := p.Dim.With(p.Italic).
		Sprintf(" (%d bookmarks)", r.Count(ctx, "bookmarks"))
	subtitle := p.Dim.With(p.Italic).
		Sprint(txt.PaddedLine("repository", name))
	header := func() string {
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	c.Frame().
		CustomFunc(header, title+comment).Ln().
		Headerln(subtitle + info).
		Rowln().Flush()

	b := bookmark.New()
	if err := parseNewBookmark(ctx, d, b, args); err != nil {
		return err
	}
	if err := bookmark.Validate(b); err != nil {
		return err
	}
	if err := saveNewBookmark(ctx, d, b); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("bookmark added\n"))
}

func HTTPStatusCodeFilter(code string) func([]*bookmark.Bookmark) []*bookmark.Bookmark {
	codes := strings.Split(strings.TrimSpace(code), ",")

	return func(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
		if len(codes) == 0 || code == "" {
			return bs
		}

		result := make([]*bookmark.Bookmark, 0, len(bs))

		for _, code := range codes {
			switch {
			// Exact status code: 200, 404, 503...
			case len(code) == 3:
				want, err := strconv.Atoi(code)
				if err != nil {
					return result
				}

				for _, b := range bs {
					if b != nil && b.HTTPStatusCode == want {
						result = append(result, b)
					}
				}

			// Status class: 2
			case len(code) == 1:
				class, err := strconv.Atoi(code)
				if err != nil {
					return result
				}

				minCode := class * 100
				maxCode := minCode + 99

				for _, b := range bs {
					if b != nil &&
						b.HTTPStatusCode >= minCode &&
						b.HTTPStatusCode <= maxCode {
						result = append(result, b)
					}
				}

			default:
				return result
			}
		}

		slices.SortFunc(result, func(a, b *bookmark.Bookmark) int {
			return cmp.Compare(a.ID, b.ID)
		})

		return result
	}
}

// parseNewBookmark fetch metadata and parses the new bookmark.
func parseNewBookmark(ctx context.Context, d *deps.Deps, b *bookmark.Bookmark, args []string) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	title := app.Flags.Title
	tags := app.Flags.TagsStr

	c := d.Console()
	newURL, err := newURLFromArgs(ctx, c, args)
	if err != nil {
		return err
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}
	if bm, exists := r.Has(ctx, newURL); exists {
		return fmt.Errorf("%w with id=%d", bookmark.ErrBookmarkDuplicate, bm.ID)
	}

	bTemp := &bookmarkTemp{}
	bTemp.title = title
	bTemp.tags = tags

	sc := scraper.New(
		newURL,
		scraper.WithSpinner("scraping webpage..."),
	)

	// fetch title, description and tags
	fetchTitleAndDesc(ctx, c, sc, bTemp)
	if err := tagsFromArgs(ctx, d, c.Term(), sc, bTemp); err != nil {
		return err
	}

	b.URL = newURL
	b.Title = bTemp.title
	b.Desc = strings.Join(txt.SplitIntoChunks(bTemp.desc, terminal.MinWidth()), "\n")
	b.Tags = bookmark.ParseTags(bTemp.tags)
	b.FaviconURL = bTemp.favicon

	return nil
}

// readURLFromClipboard checks if there a valid URL in the clipboard.
func readURLFromClipboard(ctx context.Context, c *ui.Console) string {
	cb := sys.ReadClipboard()
	if !ValidURL(cb) {
		return ""
	}

	f, p := c.Frame(), c.Palette()
	dot := func() string {
		return p.BrightMagenta.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}
	f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).
		Textln(" " + p.Gray.Sprint(cb))

	lines := txt.CountLines(f.String())
	f.Flush()

	t := c.Term()
	if err := c.ConfirmErr(ctx, "found valid URL in clipboard, use URL?", "y"); err != nil {
		t.ClearLine(lines)
		return ""
	}

	t.ClearLine(1)

	return cb
}

// newURLFromArgs parse URL from args.
func newURLFromArgs(ctx context.Context, c *ui.Console, args []string) (string, error) {
	f, t, p := c.Frame(), c.Term(), c.Palette()

	dot := func() string {
		return p.BrightMagenta.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	// checks if url is provided
	if len(args) > 0 {
		bURL := strings.TrimRight(args[0], "\n")
		f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).
			Text(" " + p.Gray.Sprint(bURL)).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	cb := readURLFromClipboard(ctx, c)
	if cb != "" {
		return cb, nil
	}

	f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).Flush()

	bURL := t.Input(" ")
	if bURL == "" {
		return bURL, metadata.ErrURLEmpty
	}

	return bURL, nil
}

func tagsFromArgs(ctx context.Context, d *deps.Deps, t tagTerminal, sc metadataScraper, b *bookmarkTemp) error {
	c := d.Console()
	f, p := c.Frame(), c.Palette()

	dot := func() string {
		return p.BrightBlue.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	f.CustomFunc(dot, p.BrightBlue.Sprint("Tags\t:"))

	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		f.Textln(" " + p.Gray.Wrap(b.tags, p.Italic)).Flush()
		return nil
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	tr := newTagResolver(r, sc, t)
	tags, err := tr.resolve(ctx, app.Flags.Force, b.tags)
	if err != nil {
		return err
	}

	t.ClearLine(1)
	b.tags = tags
	f.Textln(" " + p.Gray.Wrap(b.tags, p.Italic)).Flush()

	return nil
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(ctx context.Context, c console, sc metadataScraper, b *bookmarkTemp) {
	f, p := c.Frame(), c.Palette()
	const indentation int = 10

	borders := f.Borders()
	width := terminal.MinWidth() - len(borders.Row)

	dot := func() string {
		return p.BrightCyan.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	if b.title != "" {
		t := p.Gray.Sprint(txt.SplitAndAlign(b.title, width, indentation))
		f.CustomFunc(dot, p.BrightCyan.Sprint("Title\t: ")).Textln(t).Flush()

		return
	}

	// scrape data
	_ = sc.Start(ctx)
	b.title, _ = sc.Title()
	b.desc, _ = sc.Desc()
	b.favicon, _ = sc.Favicon()

	// title
	t := p.Gray.Sprint(txt.SplitAndAlign(b.title, width, indentation))
	f.CustomFunc(dot, p.BrightCyan.Sprint("Title\t: ")).Textln(t)

	// description
	if b.desc != "" {
		descColor := p.Gray.Sprint(txt.SplitAndAlign(b.desc, width, indentation))
		dot := func() string {
			return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
		}
		f.CustomFunc(dot, p.BrightYellow.Sprint("Desc\t: ")).Textln(descColor)
	}

	// tags
	if b.tags == "" {
		if keywords, _ := sc.Keywords(); keywords != "" {
			b.tags = keywords
		}

		// codeberg, gitlab, github
		if tags, _ := sc.TagsRepo(); len(tags) > 0 {
			tags, _ := sc.TagsRepo()
			b.tags = strings.Join(tags, ",")
		}
	}

	f.Flush()
}

// saveNewBookmark asks the user if they want to save the bookmark.
func saveNewBookmark(ctx context.Context, d *deps.Deps, b *bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if app.Flags.Force {
		return r.InsertMany(ctx, []*bookmark.Bookmark{b})
	}

	c := d.Console()
	opt, err := c.Choose(ctx, "save bookmark?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	switch strings.ToLower(opt) {
	case "n", "no":
		return sys.ErrActionAborted
	case "e", "edit":
		session := editor.NewEditSession().
			WithStrategy(editor.NewBookmarkStrategy()).
			WithPersistFunc(func(ctx context.Context, old, fresh *bookmark.Bookmark) error {
				return insertAndAddBookmark(ctx, r, app, fresh)
			})
		return runEditSession(ctx, d, []*bookmark.Bookmark{b}, session)
	default:
		return insertAndAddBookmark(ctx, r, app, b)
	}
}

// insertAndAddBookmark inserts a bookmark and applies git add.
func insertAndAddBookmark(ctx context.Context, r bookmarkStore, app *application.App, b *bookmark.Bookmark) error {
	newID, err := r.InsertOne(ctx, b)
	if err != nil {
		return err
	}

	fresh, err := r.ByID(ctx, int(newID))
	if err != nil {
		return err
	}

	if !app.GitEnabled() {
		return nil
	}

	return gitops.Add(ctx, app, r, fresh)
}

type tagResolver struct {
	store   tagStore
	scraper metadataScraper
	term    tagTerminal
}

func newTagResolver(r tagStore, sc metadataScraper, t tagTerminal) *tagResolver {
	return &tagResolver{
		store:   r,
		scraper: sc,
		term:    t,
	}
}

func (tr *tagResolver) resolve(ctx context.Context, force bool, initial string) (string, error) {
	if initial != "" {
		return bookmark.ParseTags(initial), nil
	}

	_ = tr.scraper.Start(ctx)

	if keywords, _ := tr.scraper.Keywords(); keywords != "" {
		return bookmark.ParseTags(keywords), nil
	}

	if force {
		return bookmark.DefaultTag, nil
	}

	tags, err := tr.store.TagsCounter(ctx)
	if err != nil {
		return "", err
	}

	selected := tr.term.ChooseTags(txt.GlyphSmallSquare.Prefix(" Tags  : "), tags)

	return bookmark.ParseTags(selected), nil
}
