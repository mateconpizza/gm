package gitops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

var _ bookio.FileManager = (*files.FileManager)(nil)

func CommitChanges(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()

	sess := app.Session().
		WithConfirmation(true).
		WithValidation(true)

	gm := app.Git.Manager()
	gr := NewRepo(gm, app.DBBaseName(), sess, app.Flags.Color)
	RegisterHooks(r, app, gr)

	return gm.Commit(ctx,
		gr.CommitMsg(git.RepoAction("commit changes"), ""),
	)
}

// Sync stages tracked bookmark data and commits any resulting changes to Git.
func Sync(ctx context.Context, app *application.App, gm *git.Mgr, msg git.CommitMessage) error {
	log := app.Logger().With("fn", "git-sync", "db", app.DBBaseName())

	log.Debug("starting")

	if !app.GitEnabled() {
		log.Warn("git disabled in app config, skipping")
		return nil
	}

	if !gm.IsEnabled() {
		log.Debug("git manager disabled, skipping")
		return nil
	}

	if !gm.IsTracked(app.DBBaseName()) {
		log.Debug("database not tracked in git, skipping")
		return nil
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return fmt.Errorf("git sync: failed to open database: %w", err)
	}
	defer r.Close()

	bs, err := r.All(ctx)
	if err != nil {
		return fmt.Errorf("git sync: failed to fetch bookmarks: %w", err)
	}
	log.Debug("loaded bookmarks", "count", len(bs))

	sess := app.Session().
		WithConfirmation(false).
		WithValidation(true)

	gr := NewRepo(gm, app.DBBaseName(), sess, app.Flags.Color)
	RegisterHooks(r, app, gr)

	if err := gr.Add(ctx, bs); err != nil {
		return fmt.Errorf("git sync: failed to add bookmarks: %w", err)
	}

	log.Debug("committing changes")
	return gm.Commit(ctx, msg)
}

func SyncAll(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("fn", "git-sync-all")

	c := d.Console()
	w, p := c.Writer(), c.Palette()

	gm := app.Git.Manager()
	repos := gm.Repos()
	files.PrioritizeFile(repos, app.DBBaseName())
	log.Debug("syncing repos", "count", len(repos))

	sess := app.Session().
		WithConfirmation(false).
		WithValidation(true)

	for _, name := range repos {
		if err := ctx.Err(); err != nil {
			return err
		}

		rlog := log.With("repo", name)

		var sb strings.Builder
		app.Git.SetWriter(&sb)

		path := filepath.Join(app.Path.Home(), name)
		path = files.EnsureExt(path, "db")
		if !files.Exists(path) {
			rlog.Debug("database file not found, skipping", "path", path)
			continue
		}

		r, err := db.New(ctx, path)
		if err != nil {
			return fmt.Errorf("%w: %q", err, path)
		}

		bs, err := r.All(ctx)
		if err != nil {
			return err
		}
		rlog.Debug("loaded bookmarks", "count", len(bs))

		gr := NewRepo(gm, r.BaseName(), sess, app.Flags.Color)

		if err := PruneRepo(ctx, gm, gr, bs); err != nil {
			if errors.Is(err, git.ErrGitUpToDate) {
				rlog.Debug("repo up-to-date")
				fmt.Fprintf(w, "git: repo %s up-to-date\n", p.BrightYellow.Sprint(name))
				continue
			}
			return err
		}

		rlog.Debug("repo synced")
		fmt.Fprint(w, sb.String())
	}

	fmt.Fprintln(w, git.ErrGitUpToDate.Error())

	return nil
}

func StreamLog(ctx context.Context, g *git.Git, p *ansi.Palette) error {
	logger := &git.LogStyle{
		Hash:         p.BrightYellow.Format,
		Repo:         p.BrightGreen.Format,
		Message:      func(s string) string { return s },
		Status:       p.Gray.With(p.Italic).Format,
		Info:         p.BrightBlue.Format,
		Highlight:    p.Magenta.Format,
		PreProcessor: newHighlighter(p),
	}

	status, err := g.Output(ctx, "log", "--oneline", "--reverse")
	if err != nil {
		return err
	}

	e := git.NewLogEntry().
		WithStyler(logger)

	if err := git.StreamLogs(ctx, strings.NewReader(status), g.Writer(), e); err != nil {
		if errors.Is(err, context.Canceled) {
			return application.ErrExitFailure
		}
		return err
	}

	return nil
}

func Cipher(app *application.App) func(ctx context.Context) (enc.Cipher, error) {
	return func(ctx context.Context) (enc.Cipher, error) {
		root := app.Path.Git()

		rc, err := resolveConfig(root)
		if err != nil {
			return nil, fmt.Errorf("load repo config: %w", err)
		}

		return app.Session().NewCipher(ctx, root, app.DBBaseName(), rc.Encryption)
	}
}

func BookmarksFromStore(r store) BookmarksLoader {
	return func(ctx context.Context) ([]*bookmark.Bookmark, error) {
		return r.All(ctx)
	}
}

// StatsFromStore adapts a database to a StatsLoader.
func StatsFromStore(r store) StatsLoader {
	return func(ctx context.Context) (*git.RepoStats, error) {
		s := git.NewRepoStats()
		if err := r.Stats(ctx, s); err != nil {
			return nil, err
		}
		s.Name = r.BaseName()

		return s, nil
	}
}

// NewReadmeFile renders the README markdown to w.
func NewReadmeFile(w io.Writer, cfg *ReadmeCfg, stats *git.RepoStats, bs []*bookmark.Bookmark) error {
	return bookio.NewMarkdown(
		bs,
		bookio.WithTitle(fmt.Sprintf("Bookmarks from %q", cfg.Name)),
		bookio.WithWriter(w),
		bookio.WithDecorator(bookio.MarkdownGlyphsDecorator(cfg.Glyphs)),
		bookio.WithSorter(func(bs []*bookmark.Bookmark) { _, _ = bookmark.Sort(cfg.Sort, bs) }),
		bookio.WithSummary(bookio.MarkdownSummaryFmt(cfg.Summary, &bookio.MarkdownSummary{
			Bookmarks:   stats.Bookmarks,
			Tags:        stats.Tags,
			Favorites:   stats.Favorites,
			Archived:    stats.Archived,
			DeadLinks:   stats.DeadLinks,
			TotalVisits: stats.TotalVisits,
		})),
	).Render()
}

var actionRe = regexp.MustCompile(`^(add|update|del|edit|import|sync|http|wayback|commit|params|gpg|rename|initialized|yaml|json)\b`)

// newHighlighter returns a function that closes over the pre-computed map.
func newHighlighter(p *ansi.Palette) func(string) string {
	replacements := map[string]string{
		"add":         p.BrightCyan.Sprint("add"),
		"update":      p.BrightBlue.Sprint("update"),
		"del":         p.BrightRed.Sprint("del"),
		"edit":        p.BrightYellow.Sprint("edit"),
		"import":      p.BrightYellow.Sprint("import"),
		"sync":        p.Cyan.Sprint("sync"),
		"http":        p.Magenta.Sprint("http"),
		"wayback":     p.Red.Sprint("wayback"),
		"commit":      p.BrightMagenta.Sprint("commit"),
		"params":      p.Blue.Sprint("params"),
		"gpg":         p.Red.Sprint("gpg"),
		"yaml":        p.BrightYellow.Sprint("yaml"),
		"json":        p.Green.Sprint("json"),
		"rename":      p.BrightYellow.Sprint("rename"),
		"initialized": p.Red.Sprint("initialized"),
	}

	return func(msg string) string {
		return actionRe.ReplaceAllStringFunc(msg, func(v string) string {
			return replacements[v]
		})
	}
}
