package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/metadata"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/scraper"
)

type clipboardReader func(ctx context.Context, c console) string

type takeInput func(prompt string) string

type tagStore interface {
	TagsCounter(ctx context.Context) (map[string]int, error)
}

type bookmarkStore interface {
	BaseName() string
	Stats(ctx context.Context, dest any) error
	All(ctx context.Context) ([]*bookmark.Bookmark, error)
	ByID(ctx context.Context, bID int) (*bookmark.Bookmark, error)
	InsertOne(ctx context.Context, b *bookmark.Bookmark) (int64, error)
	UpdateOne(ctx context.Context, b *bookmark.Bookmark) error
	Close()
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
	ChooseTags(ctx context.Context, prompt string, items map[string]int) (string, error)
	ClearLine(n int)
}

type console interface {
	Frame() *frame.Frame
	Palette() *ansi.Palette
	Term() *terminal.Term
	Writer() io.Writer
	Glyphs() *formatter.Glyphs
	NewBannerBuilder() *ui.BannerConfig

	Confirm(ctx context.Context, q, def string) bool
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
	name := p.BrightYellow.With(p.Bold).Sprint(r.BaseName())
	info := p.Dim.With(p.Italic).Sprintf(" (%d bookmarks)", r.Count(ctx, "bookmarks"))
	subtitle := p.Dim.With(p.Italic).Sprint(txt.PaddedLine("repository", name)) + info

	c.NewBannerBuilder().
		WithTitle("Add Bookmark").
		WithTitleGlyph(c.Glyphs().Add).
		WithComment(" (ctrl-c to exit)").
		WithSubtitle(subtitle).
		Build().
		Rowln().
		Flush()

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

// parseNewBookmark fetch metadata and parses the new bookmark.
func parseNewBookmark(ctx context.Context, d *deps.Deps, b *bookmark.Bookmark, a []string) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("fn", "parse-new-bookmark")

	title := app.Flags.Title
	tags := app.Flags.TagsStr

	c := d.Console()
	newURL, err := newURLFromArgs(ctx, c, a, c.Term().Input, readURLFromClipboard, log)
	if err != nil {
		log.Error("failed to resolve URL from arguments", "error", err)
		return err
	}

	log = log.With("url", newURL)

	r, err := d.Repository()
	if err != nil {
		return err
	}
	if bm, exists := r.Has(ctx, newURL); exists {
		log.Warn("bookmark already exists", "existing_id", bm.ID)
		return fmt.Errorf("%w with id=%d", bookmark.ErrBookmarkDuplicate, bm.ID)
	}

	bTemp := &bookmarkTemp{}
	bTemp.title = title
	bTemp.tags = tags

	sc := scraper.New(
		newURL,
		scraper.WithSpinner(NewSpinner(app.Flags.Color)),
		scraper.WithLogger(log),
	)

	// fetch title, description and tags
	fetchTitleAndDesc(ctx, c, sc, bTemp, log)
	if err := tagsFromArgs(ctx, d, c.Term(), sc, bTemp, log); err != nil {
		log.Error("failed to parse tags", "error", err)
		return err
	}

	b.URL = newURL
	b.Title = bTemp.title
	b.Desc = strings.Join(txt.SplitIntoChunks(bTemp.desc, c.Term().MinWidth()), "\n")
	b.Tags = bookmark.ParseTags(bTemp.tags)
	b.FaviconURL = bTemp.favicon

	log.Debug("bookmark successfully parsed", "title", b.Title, "tags_count", len(strings.FieldsFunc(b.Tags, func(r rune) bool {
		return r == ',' || r == ' '
	})))

	return nil
}

// readURLFromClipboard checks if there a valid URL in the clipboard.
func readURLFromClipboard(ctx context.Context, c console) string {
	cb := sys.ReadClipboard()
	if !ValidURL(cb) {
		return ""
	}

	f, p := c.Frame(), c.Palette()
	dot := func() string {
		return p.BrightMagenta.Wrap(c.Glyphs().Square+" ", p.Bold)
	}
	f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).
		Textln(" " + p.Gray.Sprint(cb))

	lines := txt.CountLines(f.String())
	f.Flush()

	t := c.Term()
	if err := c.Term().ConfirmErr(ctx, "found valid URL in clipboard, use URL?", "y"); err != nil {
		t.ClearLine(lines)
		return ""
	}

	t.ClearLine(1)

	return cb
}

// newURLFromArgs parse URL from args.
func newURLFromArgs(ctx context.Context, c console, args []string, ti takeInput, cr clipboardReader, log *slog.Logger) (string, error) {
	log = log.With("fn", "new-url-from-args")

	f, p := c.Frame(), c.Palette()

	dot := func() string {
		return p.BrightMagenta.Wrap(c.Glyphs().Square+" ", p.Bold)
	}

	// checks if url is provided
	if len(args) > 0 {
		bURL := strings.TrimRight(args[0], "\n")
		log.Debug("url from args", "nargs", len(args), "length", len(bURL))

		f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).
			Text(" " + p.Gray.Sprint(bURL)).Ln().Flush()

		return bURL, nil
	}

	// checks clipboard
	cb := cr(ctx, c)
	if cb != "" {
		log.Debug("url from clipboard", "length", len(cb))
		return cb, nil
	}
	log.Debug("clipboard empty, prompting")

	if err := ctx.Err(); err != nil {
		log.Debug("context done before prompt", "cause", context.Cause(ctx))
		return "", err
	}

	f.CustomFunc(dot, p.BrightMagenta.Sprint("URL\t:")).Flush()

	bURL := ti(" ")
	if bURL == "" {
		log.Debug("empty url from prompt")
		return bURL, metadata.ErrURLEmpty
	}
	log.Debug("url from prompt", "length", len(bURL))

	return bURL, nil
}

func tagsFromArgs(ctx context.Context, d *deps.Deps, t tagTerminal, sc metadataScraper, b *bookmarkTemp, log *slog.Logger) error {
	log = log.With("fn", "tags-from-args")

	c := d.Console()
	f, p := c.Frame(), c.Palette()

	dot := func() string {
		return p.BrightBlue.Wrap(c.Glyphs().Square+" ", p.Bold)
	}

	f.CustomFunc(dot, p.BrightBlue.Sprint("Tags\t:"))

	if b.tags != "" {
		b.tags = bookmark.ParseTags(b.tags)
		log.Debug("using provided tags")
		f.Textln(" " + p.Gray.Wrap(b.tags, p.Italic)).Flush()
		return nil
	}

	r, err := d.Repository()
	if err != nil {
		log.Error("failed to get repository", "error", err)
		return err
	}

	app, err := d.Application(ctx)
	if err != nil {
		log.Error("failed to get application", "error", err)
		return err
	}

	log.Debug("resolving tags", "force", app.Flags.Force)

	tr := newTagResolver(r, sc, t, log)
	tags, err := tr.resolve(ctx, app.Flags.Force, b.tags, c.Glyphs().Square)
	if err != nil {
		log.Error("failed to resolve tags", "error", err)
		return err
	}

	t.ClearLine(1)
	b.tags = tags

	log.Debug("tags resolved")
	f.Textln(" " + p.Gray.Wrap(b.tags, p.Italic)).Flush()

	return nil
}

// fetchTitleAndDesc fetch and display title and description.
func fetchTitleAndDesc(ctx context.Context, c console, sc metadataScraper, b *bookmarkTemp, log *slog.Logger) {
	log = log.With("fn", "fetch-title-and-desc")

	f, p := c.Frame(), c.Palette()
	const indentation int = 10

	borders := f.Borders()
	width := c.Term().MinWidth() - len(borders.Row)

	dot := func() string {
		return p.BrightCyan.Wrap(c.Glyphs().Square+" ", p.Bold)
	}

	if b.title != "" {
		log.Debug("using provided title")
		t := p.Gray.Sprint(txt.SplitAndAlign(b.title, width, indentation))
		f.CustomFunc(dot, p.BrightCyan.Sprint("Title\t: ")).Textln(t).Flush()
		return
	}

	log.Debug("scraping page metadata")

	if err := sc.Start(ctx); err != nil {
		log.Debug("failed to start metadata scraper", "error", err)
	}

	var err error
	b.title, err = sc.Title()
	if err != nil {
		log.Debug("failed to fetch title", "error", err)
	}

	b.desc, err = sc.Desc()
	if err != nil {
		log.Debug("failed to fetch description", "error", err)
	}

	b.favicon, err = sc.Favicon()
	if err != nil {
		log.Debug("failed to fetch favicon", "error", err)
	}

	log.Debug("page metadata fetched",
		"has_title", b.title != "",
		"has_description", b.desc != "",
		"has_favicon", b.favicon != "",
	)

	// title
	t := p.Gray.Sprint(txt.SplitAndAlign(b.title, width, indentation))
	f.CustomFunc(dot, p.BrightCyan.Sprint("Title\t: ")).Textln(t)

	// description
	if b.desc != "" {
		descColor := p.Gray.Sprint(txt.SplitAndAlign(b.desc, width, indentation))
		dot := func() string {
			return p.BrightYellow.Wrap(c.Glyphs().Square+" ", p.Bold)
		}
		f.CustomFunc(dot, p.BrightYellow.Sprint("Desc\t: ")).Textln(descColor)
	}

	// tags
	if b.tags == "" {
		keywords, err := sc.Keywords()
		if err != nil {
			log.Debug("failed to fetch keywords", "error", err)
		} else if keywords != "" {
			b.tags = keywords
			log.Debug("using page keywords")
		}

		// codeberg, gitlab, github
		tags, err := sc.TagsRepo()
		if err != nil {
			log.Debug("failed to fetch repository tags", "error", err)
		} else if len(tags) > 0 {
			b.tags = strings.Join(tags, ",")
			log.Debug("using repository tags", "count", len(tags))
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

	log := app.Logger().With("fn", "save-new-bookmark", "url", b.URL)

	if app.Flags.Force {
		log.Debug("force flag enabled, inserting bookmark directly")
		return r.InsertMany(ctx, []*bookmark.Bookmark{b})
	}

	c := d.Console()
	opt, err := c.Choose(ctx, "save bookmark?", []string{"yes", "no", "edit"}, "y")
	if err != nil {
		log.Error("failed to prompt save choice", "error", err)
		return fmt.Errorf("%w", err)
	}

	log = log.With("user_choice", opt)

	switch strings.ToLower(opt) {
	case "no":
		log.Debug("user aborted bookmark save")
		return app.Abort()
	case "edit":
		log.Debug("user chose to edit bookmark before saving")
		e := editor.NewBookmarkStrategy().
			WithRepoName(app.DBBaseName()).
			WithAppName(app.Name).
			WithAppVersion(app.Version()).
			WithSpinner(NewSpinner(app.Flags.Color))
		session := editor.NewEditSession().
			WithStrategy(e).
			WithDiffer(c.Differ()).
			WithLogger(app.Logger()).
			WithPersistFunc(func(ctx context.Context, old, fresh *bookmark.Bookmark) error {
				return insertAndAddBookmark(ctx, r, app, fresh)
			})

		return runEditSession(ctx, d, []*bookmark.Bookmark{b}, session)
	default:
		log.Debug("saving bookmark normally")
		return insertAndAddBookmark(ctx, r, app, b)
	}
}

// insertAndAddBookmark inserts a bookmark and applies git add.
func insertAndAddBookmark(ctx context.Context, r bookmarkStore, app *application.App, b *bookmark.Bookmark) error {
	log := app.Logger().With("fn", "insert-and-add-bookmark", "url", b.URL)

	newID, err := r.InsertOne(ctx, b)
	if err != nil {
		log.Error("failed to insert bookmark", "error", err)
		return err
	}

	log = log.With("bookmark_id", newID)

	fresh, err := r.ByID(ctx, int(newID))
	if err != nil {
		log.Error("failed to retrieve inserted bookmark by ID", "error", err)
		return err
	}

	if !app.GitEnabled() {
		log.Debug("git integration disabled, skipping git add")
		return nil
	}

	log.Debug("adding bookmark to git repository")

	sess := app.Session().
		WithConfirmation(true).
		WithValidation(true)

	gm, err := app.Git.Manager()
	if err != nil {
		return err
	}
	gr := gitops.NewRepo(gm, app.DBBaseName(), sess, app.Flags.Color)
	gitops.RegisterHooks(r, app, gm, gr)

	if err := gitops.Add(ctx, gm, gr, fresh); err != nil {
		log.Error("failed to add bookmark to git", "error", err)
		return err
	}

	log.Debug("bookmark successfully inserted and added to git")
	return nil
}

type tagResolver struct {
	store   tagStore
	scraper metadataScraper
	term    tagTerminal
	logger  *slog.Logger
}

func newTagResolver(r tagStore, sc metadataScraper, t tagTerminal, log *slog.Logger) *tagResolver {
	return &tagResolver{
		store:   r,
		scraper: sc,
		term:    t,
		logger:  log,
	}
}

func (tr *tagResolver) resolve(ctx context.Context, force bool, initial, glyph string) (string, error) {
	log := tr.logger.With("fn", "tag-resolver")

	if initial != "" {
		log.Debug("using provided tags")
		return bookmark.ParseTags(initial), nil
	}

	if err := tr.scraper.Start(ctx); err != nil {
		log.Debug("failed to start metadata scraper", "error", err)
	}

	keywords, err := tr.scraper.Keywords()
	if err != nil {
		log.Debug("failed to fetch keywords", "error", err)
	} else if keywords != "" {
		log.Debug("using page keywords")
		return bookmark.ParseTags(keywords), nil
	}

	if force {
		log.Debug("using default tags")
		return bookmark.DefaultTag, nil
	}

	tags, err := tr.store.TagsCounter(ctx)
	if err != nil {
		log.Error("failed to retrieve existing tags", "error", err)
		return "", err
	}

	log.Debug("retrieved existing tags", "count", len(tags))

	selected, err := tr.term.ChooseTags(ctx, glyph+" Tags  : ", tags)
	if err != nil {
		log.Error("failed to choose tags", "error", err)
		return "", err
	}

	log.Debug("tags selected")

	return bookmark.ParseTags(selected), nil
}
