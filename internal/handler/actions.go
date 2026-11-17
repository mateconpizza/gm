// Package handler handles parsing and processing of bookmark data operations.
package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	menu "github.com/mateconpizza/go-fzf"
	files "github.com/mateconpizza/gofiles"
	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/errgroup"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/bookmark/status"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/session"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
	"github.com/mateconpizza/gm/pkg/scraper"
)

func NewSpinner(color bool) *rotato.Rotato {
	p := ansi.NewPalette(color)

	return rotato.New(
		rotato.WithMessage("scraping webpage..."),
		rotato.WithMessageColor(p.Yellow.Format),
		rotato.WithSpinnerColor(p.BrightMagenta.Format),
	)
}

// Open opens the URLs in the browser for the bookmarks in the provided slice.
func Open(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	const maxGoroutines = 5
	n := len(bs)
	p := d.Console().Palette()
	s := fmt.Sprintf("%s %d bookmarks", p.BrightGreen.Wrap("open", p.Bold), n)

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("op", "open", "count", n)
	log.Debug("starting", "force", app.Flags.Force)

	if err := d.Console().ConfirmLimit(ctx, n, maxGoroutines, s, app.Flags.Force); err != nil {
		log.Debug("aborted at confirmation", "err", err)
		return err
	}

	sp := NewSpinner(app.Flags.Color)
	sp.UpdateMesg("opening bookmarks...")
	sp.UpdateSymbols(rotato.WithSpinnerStyle(rotato.SpinnerPulse))

	if err := openInBrowser(ctx, &openOpts{
		spinner:   sp,
		bookmarks: bs,
		opener:    sys.OpenInBrowser,
		logger:    log,
	}); err != nil {
		log.Error("open failed", "err", err)
		return err
	}
	log.Info("bookmarks opened")

	if err := recordVisits(ctx, d, bs); err != nil {
		log.Error("record visits failed after open", "err", err)
		return fmt.Errorf("record visits: %w", err)
	}
	log.Debug("visits recorded")

	return nil
}

// Edit returns a BookmarkAction configured with a specific strategy.
func Edit(ctx context.Context, strategy editor.EditStrategy) func(context.Context, *deps.Deps, []*bookmark.Bookmark) error {
	return func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
		const maxItems = 10

		app, err := d.Application(ctx)
		if err != nil {
			return err
		}

		c := d.Console()
		p := c.Palette()
		q := fmt.Sprintf("%s %d bookmarks", p.BrightGreen.Wrap("edit", p.Bold), len(bs))

		if err := c.ConfirmLimit(ctx, len(bs), maxItems, q, app.Flags.Force); err != nil {
			return err
		}

		r, err := d.Repository()
		if err != nil {
			return err
		}
		defer r.Close()

		es := editor.NewEditSession().
			WithStrategy(strategy).
			WithDiffer(c.Differ()).
			WithLogger(app.Logger()).
			WithPersistFunc(func(ctx context.Context, old, fresh *bookmark.Bookmark) error {
				return persistFunc(ctx, app, PersistParams{
					Repo:  r,
					Old:   old,
					Fresh: fresh,
					Msg: git.NewRepo(r.BaseName(), "").
						CommitMsg(git.Edit, "bookmark"),
				})
			})

		return runEditSession(ctx, d, bs, es)
	}
}

func Yank(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	c := d.Console()
	p := c.Palette()

	msg := fmt.Sprintf(
		"%s %d bookmarks to system clipboard",
		p.BrightGreen.Wrap("copy", p.Bold),
		len(bs),
	)

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if err := c.ConfirmLimit(ctx, len(bs), 10, msg, app.Flags.Force); err != nil {
		return err
	}

	content, err := clipboardContent(bs, app.Flags.JSON)
	if err != nil {
		return err
	}

	if err := sys.CopyClipboard(content); err != nil {
		return err
	}

	return c.Print(
		ctx,
		c.SuccessMesg("copied ", len(bs), " bookmarks to system clipboard\n"),
	)
}

// HTTPStatusCheck refreshes and persists bookmark HTTP status information.
func HTTPStatusCheck(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return bookmark.ErrBookmarkNotFound
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("op", "status.check", "count", len(bs))
	log.Debug("starting", "force", app.Flags.Force)

	c, p := d.Console(), d.Console().Palette()
	q := fmt.Sprintf("checking %s of %d bookmarks", p.BrightGreen.Wrap("status", p.Bold), len(bs))
	if err = c.ConfirmLimit(ctx, len(bs), 15, q, app.Flags.Force); err != nil {
		log.Debug("aborted at confirmation", "err", err)
		return app.Abort()
	}

	bs, err = status.Check(ctx, c, bs, log)
	if err != nil {
		log.Error("status check failed", "err", err)
		return err
	}

	if err := saveStatusUpdates(ctx, d, bs); err != nil {
		log.Error("saving status updates failed", "updated", len(bs), "err", err)
		return fmt.Errorf("save status updates: %w", err)
	}
	log.Info("status updates saved", "updated", len(bs))

	return c.Print(
		ctx,
		d.Console().SuccessMesg("bookmark status checked\n"),
	)
}

// HTTPStatus prints bookmark HTTP status information.
func HTTPStatus(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return nil
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("fn", "http-status", "count", len(bs))

	if app.Flags.Field != "" {
		log.Debug("field output requested, skipping summary table", "field", app.Flags.Field)
		return printer.Display(ctx, d.Console(), formatter.HTTPStatusCode.String(), bs)
	}

	headers := []string{"Code", "Count", "Description"}
	rows := [][]string{}
	footer := []string{}

	type Row struct {
		description string
		count       int
	}

	newItems := make(map[int]Row, len(bs))
	c := d.Console()
	p := c.Palette()

	for i := range bs {
		b := bs[i]
		statusText := b.HTTPStatusText
		if statusText == "" {
			statusText = "Unassigned"
		}

		r := newItems[b.HTTPStatusCode]
		r.description = txt.HTTPStatusCodeColor(b.HTTPStatusCode, p).Sprint(statusText)
		r.count++
		newItems[b.HTTPStatusCode] = r
	}

	codes := make([]int, 0, len(newItems))
	for code := range newItems {
		codes = append(codes, code)
	}

	sort.Ints(codes)
	log.Debug("status summary built", "distinct_codes", len(codes), "unassigned", newItems[0].count)

	for _, code := range codes {
		v := newItems[code]

		rows = append(rows, []string{
			strconv.Itoa(code),
			strconv.Itoa(v.count),
			v.description,
		})
	}

	return c.Print(ctx, txt.CreateSimpleTable(headers, rows, footer...))
}

// UpdateMetadata refreshes metadata for the provided bookmarks.
func UpdateMetadata(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	c, p := d.Console(), d.Console().Palette()

	s := fmt.Sprintf("update metadata of %d bookmarks", len(bs))
	if err := c.ConfirmLimit(ctx, len(bs), 10, s, app.Flags.Force); err != nil {
		return app.Abort()
	}

	n := len(bs)
	if n > 1 {
		c.NewBannerBuilder().
			WithTitle(fmt.Sprintf("Updating %d bookmarks", n)).
			WithTitleGlyph(c.Glyphs().Update).
			WithTitleColor(p.BrightGreen.With(p.Bold).Sprint).
			WithSubtitle("metadata: title, desc").
			WithComment(" (ctrl-c to exit)").Build().
			Rowln().
			Flush()
	}

	for _, b := range bs {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := processMetadataUpdate(ctx, d, b); err != nil {
			return err
		}
	}

	return nil
}

func RemoveRepos(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("fn", "remove-repos")

	gm, err := app.Git.Manager()
	if err != nil {
		return err
	}

	c := d.Console()
	p := c.Palette()
	boldRed := p.BrightRed.With(p.Bold)

	items, err := dbops.NewDatabaseSelector(app).
		WithItemDecorator(gitTrackedMarker(app.Glyphs(), gm.IsTracked)).
		WithOpts(
			menu.WithMultiSelection(),
			menu.WithHeaderLabel(" remove database "),
			menu.WithColor("header-label", menu.ColorBrightRed, menu.AttributeBold),
			menu.WithHeader(fmt.Sprintf(
				"select database %s %s",
				app.Glyphs().Sep,
				boldRed.Sprint("this action cannot be undone"),
			)),
		).
		Select(ctx)
	if err != nil {
		return err
	}
	log.Debug("databases selected", "count", len(items))

	value := func(s string) string {
		return p.BrightYellow.With(p.Italic).Sprint(
			files.CollapseHomeDir(s),
		)
	}

	c.NewBannerBuilder().
		WithTitle("Remove Database/s").
		WithTitleGlyph(c.Glyphs().Warning).
		WithTitleColor(p.BrightRed.With(p.Bold).Sprint).
		WithSubtitle("this action cannot be undone").
		Build().
		Rowln().
		Info("database: " + value(app.Path.DB()+"\n")).
		Rowln().
		Flush()

	for i := range items {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Debug("canceled by user", "done", i, "total", len(items))
				return app.Abort()
			}
			return err
		}

		if err := removeOneDB(ctx, c, gm, items[i], log); err != nil {
			return err
		}
	}

	log.Debug("remove finished", "total", len(items))

	return nil
}

func DropRepo(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()

	if err := dbops.Drop(ctx, d); err != nil {
		return err
	}

	gm, err := app.Git.Manager()
	if err != nil {
		return err
	}
	ops := gitops.NewRepoOps(app.Session(), gitops.WithRepoOpsLogger(app.Logger()))
	gr := gm.NewRepo(r.Name(), ops.Remover())
	gitops.RegisterHooks(r, app, gm, gr)

	return gitops.Drop(ctx, gm, gr, d.Console(), app.Logger())
}

func Rename(ctx context.Context, d *deps.Deps, name string, renameFunc func(src, dest string) error) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	// names and file paths
	oldName := app.DBBaseName()
	newName := files.EnsureExt(app.Flags.Rename, ".db")

	srcDBPath := app.Path.DB()
	destDBPath := filepath.Join(app.Path.Data, newName)

	log := app.Logger().With("op", "rename", "old", oldName, "new", newName)
	log.Debug("starting", "src", srcDBPath, "dest", destDBPath)

	if files.Exists(destDBPath) {
		log.Error("destination already exists", "dest", destDBPath)
		return fmt.Errorf("%w: %q", os.ErrExist, app.Flags.Rename)
	}

	// prompt for confirmation
	c := d.Console()

	showRenameBanner(c, app, srcDBPath, destDBPath, newName)

	if !c.Confirm(ctx, "continue?", "n") {
		log.Debug("declined by user")
		return app.Abort()
	}

	// close connection
	oldRepo, err := d.Repository()
	if err != nil {
		return err
	}
	oldRepo.Close()
	log.Debug("connection closed")

	// database file
	if err := renameFunc(srcDBPath, destDBPath); err != nil {
		log.Error("move database file failed", "err", err)
		return fmt.Errorf("failed to move database file: %w", err)
	}
	log.Info("database file renamed")

	if !git.IsInitialized(app.Path.Git()) {
		log.Debug("git not initialized, skipping git rename")
		return nil
	}

	gm, err := app.Git.Manager()
	if err != nil {
		log.Error("git manager failed after db rename", "err", err)
		return fmt.Errorf("database renamed but git sync failed: %w", err)
	}

	if err := gm.Rename(ctx, oldName, newName); err != nil {
		log.Error("git rename failed, db renamed but git repo not", "err", err)
		return fmt.Errorf("database renamed but git sync failed: %w", err)
	}
	log.Info("git repo renamed")

	return nil
}

type MarkdownCfg struct {
	Title     string
	Summary   string
	Glyphs    string
	Sort      string
	Bookmarks []*bookmark.Bookmark
}

type isTracked func(name string) bool

func gitTrackedMarker(g *formatter.Glyphs, f isTracked) func(string) string {
	return func(s string) string {
		name, _, _ := strings.Cut(ansi.Remover(s), " ")

		var icon strings.Builder
		if f(name) {
			icon.WriteString(ansi.BrightYellow.Sprint(" " + g.Git))
		}

		return s + icon.String()
	}
}

func removeOneDB(ctx context.Context, c console, gm *git.Mgr, dbPath string, log *slog.Logger) error {
	name := files.StripExts(filepath.Base(dbPath))
	log = log.With("fn", "remove-one-db", "db", name)

	p := c.Palette()
	if !c.Confirm(ctx, p.BrightRed.Wrap("remove", p.Bold)+" "+name+"?", "n") {
		log.Debug("removal declined")
		return nil
	}

	if err := os.Remove(dbPath); err != nil {
		return err
	}
	log.Debug("database file removed")

	var result strings.Builder
	result.WriteString("removed ")
	fmt.Fprintf(&result, "%q", name)

	if gm.IsTracked(name) {
		log.Debug("database was tracked in git, untracking")
		result.WriteString(" and untracked")
		gr := gitops.NewRepo(gm, name, session.New().WithLogger(log), p.Enabled())
		if err := gm.Untrack(ctx, gr); err != nil {
			return err
		}
		log.Debug("untracked from git")
	}

	fmt.Fprintln(c.Writer(), c.SuccessMesg(result.String()))

	return nil
}

type spinner interface {
	Start(ctx context.Context)
	Done(mesg ...string)
}

type openOpts struct {
	spinner   spinner
	bookmarks []*bookmark.Bookmark
	opener    func(ctx context.Context, s string) error
	logger    *slog.Logger
}

// openInBrowser concurrently opens each bookmark URL in the browser.
func openInBrowser(ctx context.Context, opts *openOpts) error {
	log := opts.logger
	opts.spinner.Start(ctx)
	defer opts.spinner.Done()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for _, b := range opts.bookmarks {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				log.Debug("skipped, context done", "url", b.URL, "err", ctx.Err())
				return ctx.Err()
			default:
				if err := opts.opener(ctx, b.URL); err != nil {
					log.Error("open url failed", "url", b.URL, "err", err)
					return fmt.Errorf("open error: %w", err)
				}
				log.Debug("opened url", "url", b.URL)

				return nil
			}
		})
	}

	return g.Wait()
}

// recordVisits increments the visit counter for each bookmark.
func recordVisits(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}

	for _, b := range bs {
		if err := r.AddVisit(ctx, b.ID); err != nil {
			return err
		}
	}

	return nil
}

func clipboardContent(bs []*bookmark.Bookmark, asJSON bool) (string, error) {
	if asJSON {
		b, err := port.ToJSON(bs)
		if err != nil {
			return "", err
		}

		return string(b), nil
	}

	var sb strings.Builder
	for _, b := range bs {
		sb.WriteString(b.URL)
		sb.WriteByte('\n')
	}

	return sb.String(), nil
}

// processMetadataUpdate updates a bookmark's metadata after user confirmation.
func processMetadataUpdate(ctx context.Context, d *deps.Deps, b *bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("fn", "process-metadata-update", "id", b.ID)

	c := d.Console()
	updated, err := updateBookmarkData(ctx, c.Palette(), b)
	if err != nil {
		return err
	}

	if changes := displayBookmarkChanges(c, b, &updated); !changes {
		log.Debug("no metadata changes")
		return nil
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}

	// handle user choice
	opt, err := c.Choose(ctx, "save changes?", []string{"yes", "no", "edit"}, "n")
	if err != nil {
		return fmt.Errorf("choose: %w", err)
	}
	log.Debug("option chosen", "option", opt)

	msg := git.NewRepo(app.DBBaseName(), "").
		CommitMsg(git.Update, "metadata")

	switch strings.ToLower(opt) {
	case "no":
		return nil

	case "yes":
		p := PersistParams{Repo: r, Old: b, Fresh: &updated, Msg: msg, OnPersisted: func() error {
			return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", updated.ID)))
		}}
		if err := persistFunc(ctx, app, p); err != nil {
			return err
		}
		log.Debug("metadata persisted")

	case "edit":
		e := editor.NewBookmarkStrategy().
			WithRepoName(app.DBBaseName()).
			WithAppName(app.Name).
			WithAppVersion(app.Version()).
			WithSpinner(NewSpinner(app.Flags.Color))

		es := editor.NewEditSession().
			WithStrategy(e).
			WithDiffer(c.Differ()).
			WithLogger(log).
			WithPersistFunc(func(ctx context.Context, old, fresh *bookmark.Bookmark) error {
				return persistFunc(ctx, app, PersistParams{
					Repo:  r,
					Old:   old,
					Fresh: fresh,
					Msg:   msg,
					OnPersisted: func() error {
						return c.Print(ctx, c.SuccessMesg(fmt.Sprintf("bookmark [%d] updated\n", updated.ID)))
					},
				})
			})

		log.Debug("opening editor session")

		if err := runEditSession(ctx, d, []*bookmark.Bookmark{&updated}, es); err != nil {
			return err
		}
	}

	return nil
}

func displayBookmarkChanges(c *ui.Console, b, updated *bookmark.Bookmark) bool {
	p := c.Palette()
	su := p.Italic.Sprint(txt.Shorten(b.URL, c.MinWidth()))

	if b.Equals(updated) {
		nc := p.Dim.With(p.Italic).Sprint(" (no changes)")
		c.Warning(p.BrightYellow.Sprint("skipping ")).
			Textln(su + nc).
			Flush()
		return false
	}

	c.Frame().
		Info(p.BrightBlue.Wrap("changes  ", p.Italic) + p.Italic.Sprint(su)).Ln().
		Textln(txt.DiffColorize(c.Differ(), txt.Diff(b.Buffer(), updated.Buffer()))).
		Flush()

	return true
}

func updateBookmarkData(ctx context.Context, p *ansi.Palette, b *bookmark.Bookmark) (bookmark.Bookmark, error) {
	updatedB := *b
	su := txt.Shorten(updatedB.URL, 60)

	sp := rotato.New(
		rotato.WithMessage("scraping "+p.Italic.Sprint(su)),
		rotato.WithSpinnerColor(p.BrightMagenta.Format),
	)

	sc := scraper.New(updatedB.URL, scraper.WithSpinner(sp))
	if err := sc.Start(ctx); err != nil {
		return updatedB, err
	}

	updatedB.Title, _ = sc.Title()
	updatedB.Desc, _ = sc.Desc()
	updatedB.FaviconURL, _ = sc.Favicon()
	return updatedB, nil
}

func runEditSession(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark, es *editor.EditSession) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	te, err := editor.NewEditor(app.Env.Editor, app.Logger())
	if err != nil {
		return err
	}

	return es.
		WithTerminal(d.Console()).
		WithEditor(te).
		Run(ctx, bs)
}

func saveStatusUpdates(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("op", "status.save", "repo", r.BaseName(), "count", len(bs))

	gm, err := app.Git.Manager()
	if err != nil {
		log.Error("git manager failed", "err", err)
		return err
	}

	sess := app.Session().
		WithConfirmation(true).
		WithValidation(true)

	gr := gitops.NewRepo(gm, r.BaseName(), sess, app.Flags.Color)
	gitops.RegisterHooks(r, app, gm, gr)

	isTracked := gm.IsTracked(gr.Name())
	log.Debug("starting", "git_tracked", isTracked)

	var updated, skipped int

	for _, b := range bs {
		if b.HTTPStatusCode == http.StatusTooManyRequests {
			log.Debug("skipped, rate limited", "url", b.URL, "id", b.ID)
			skipped++
			continue
		}

		if err := r.UpdateOne(ctx, b); err != nil {
			log.Error("db update failed",
				"url", b.URL,
				"id", b.ID,
				"updated", updated,
				"err", err,
			)
			return fmt.Errorf("update bookmark %d: %w", b.ID, err)
		}
		log.Debug("db updated", "url", b.URL, "id", b.ID, "status", b.HTTPStatusCode)

		if isTracked {
			if err := gr.Update(ctx, b, b, files.RemoveEmptyDirs); err != nil {
				log.Error("git update failed, db already updated",
					"url", b.URL,
					"id", b.ID,
					"updated", updated,
					"err", err,
				)
				return fmt.Errorf("git update bookmark %d: %w", b.ID, err)
			}
			log.Debug("git updated", "url", b.URL, "id", b.ID)
		}

		updated++
	}

	if !isTracked {
		log.Info("status saved", "updated", updated, "skipped", skipped)
		return nil
	}

	err = gm.Commit(ctx, gr.CommitMsg(git.Update, "status"))
	switch {
	case errors.Is(err, git.ErrGitUpToDate):
		log.Debug("nothing to commit")
	case err != nil:
		log.Error("git commit failed", "updated", updated, "err", err)
		return fmt.Errorf("commit status updates: %w", err)
	default:
		log.Debug("committed")
	}

	log.Info("status saved", "updated", updated, "skipped", skipped, "git", true)

	return nil
}

type PersistParams struct {
	Repo  bookmarkStore
	Old   *bookmark.Bookmark
	Fresh *bookmark.Bookmark
	Msg   git.CommitMessage

	// OnPersisted runs after the bookmark and git sync have both
	// succeeded. Optional; nil is a no-op.
	OnPersisted func() error
}

func persistFunc(ctx context.Context, app *application.App, p PersistParams) error {
	if err := p.Repo.UpdateOne(ctx, p.Fresh); err != nil {
		return err
	}

	isTracked := git.IsTracked(app.Path.Git(), app.DBBaseName())
	if !app.GitEnabled() || !isTracked {
		return runOnPersisted(p)
	}

	gm, err := app.Git.Manager()
	if err != nil {
		return err
	}

	sess := app.Session().
		WithConfirmation(false).
		WithValidation(true)

	gr := gitops.NewRepo(gm, p.Repo.BaseName(), sess, app.Flags.Color)
	gitops.RegisterHooks(p.Repo, app, gm, gr)

	if err := gm.Apply(ctx, p.Msg, func(ctx context.Context) error {
		return gr.Update(ctx, p.Old, p.Fresh, files.RemoveEmptyDirs)
	}); err != nil {
		return err
	}

	return runOnPersisted(p)
}

func runOnPersisted(p PersistParams) error {
	if p.OnPersisted != nil {
		return p.OnPersisted()
	}
	return nil
}

// showRenameBanner prints the confirmation banner for a database rename.
func showRenameBanner(c *ui.Console, app *application.App, srcName, destName, newName string) {
	p := c.Palette()

	srcNameColor := strings.Replace(srcName, app.DBName, p.BrightYellow.Sprint(app.DBName), 1)
	destNameColor := strings.Replace(destName, newName, p.BrightBlue.Sprint(newName), 1)

	c.NewBannerBuilder().
		WithTitle("Rename a database").
		WithTitleColor(p.BrightRed.Sprint).
		WithTitleGlyph(app.Glyphs().Warning).
		WithSubtitle("this action cannot be undone").
		Build().
		Rowln().
		Midln(txt.PaddedLine("From:", srcNameColor)).
		Midln(txt.PaddedLine("Dest:", destNameColor)).
		Rowln().
		Flush()
}
