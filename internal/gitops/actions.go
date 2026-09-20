package gitops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	files "github.com/mateconpizza/gofiles"
	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

var _ bookio.FileManager = (*files.FileManager)(nil)

func RepoFileReader(color bool) git.RepoOptFunc { return git.WithRepoReader(readFiles(color)) }
func RepoFileRemover() git.RepoOptFunc          { return git.WithRepoRemover(removeFiles) }
func RepoFileWriter(color bool) git.RepoOptFunc { return git.WithRepoWriter(addFiles(color)) }
func RepoStatsReader(r store) git.RepoOptFunc   { return git.WithRepoStore(r) }
func MgrVersion(ver string) git.MgrOptFunc      { return git.WithVersion(ver) }

// Init initializes Git support and configures repository encryption.
func Init(ctx context.Context, app *application.App, gm *git.Mgr) error {
	if err := gm.Init(ctx, app.Flags.Reinit); err != nil {
		if errors.Is(err, git.ErrGitInitialized) {
			p := ansi.NewPalette(app.Flags.Color)
			s := p.BrightYellow.With(p.Italic).Sprint("git init --reinit")
			return fmt.Errorf("%w, use %s", err, s)
		}
		return err
	}

	c := ui.NewDefaultConsole(app.Flags.Color, app.Exit)
	if err := AskForEncryption(ctx, c, app, gm); err != nil {
		return err
	}

	if err := c.Print(ctx, c.SuccessMesg("git initialized\n")); err != nil {
		return err
	}

	app.Git.Enabled = true
	return app.WriteConfig(true)
}

// Push pushes any unpushed commits to the configured upstream remote.
func Push(ctx context.Context, app *application.App, gm *git.Mgr) error {
	g := gm.Git()
	remote, err := g.Remote(ctx)
	if err != nil || remote == "" {
		return git.ErrGitNoUpstream
	}

	if err := g.SetUpstream(ctx, app.Path.Git()); err != nil {
		if !errors.Is(err, git.ErrGitUpstreamExists) {
			return err
		}
	}

	// Check if there are unpushed commits
	proceed, err := g.HasUnpushedCommits(ctx)
	if err != nil {
		return err
	}
	if !proceed {
		return git.ErrGitUpToDate
	}

	if err := g.Push(ctx); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// Sync stages tracked bookmark data and commits any resulting changes to Git.
func Sync(ctx context.Context, app *application.App, gm *git.Mgr, msg git.CommitMessage) error {
	slog.Debug("starting git sync")
	if !app.GitEnabled() {
		slog.Warn("git sync: disabled")
		return nil
	}

	if !gm.IsEnabled() {
		slog.Debug("git sync disabled, skipping", "enabled", gm.IsEnabled())
		return nil
	}

	if !gm.IsTracked(app.DBBaseName()) {
		slog.Debug("database path not tracked in git, skipping sync")
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

	gr := gm.NewRepo(r.Name(),
		RepoFileReader(gm.Color()),
		RepoFileRemover(),
		RepoFileWriter(gm.Color()),
		RepoStatsReader(r),
	)
	if err := gr.Add(ctx, bs); err != nil {
		return fmt.Errorf("git sync: failed to add bookmarks: %w", err)
	}

	return gm.SaveChanges(ctx, gr, msg)
}

func SyncAll(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	gm, err := NewManager(&ManagerConfig{
		Root:    app.Path.Git(),
		Writer:  app.Git.Writer(),
		Version: app.Version(),
		Color:   app.Flags.Color,
	})
	if err != nil {
		return err
	}

	c := d.Console()
	w := c.Writer()
	p := c.Palette()

	for _, name := range gm.Repos() {
		if err := ctx.Err(); err != nil {
			return err
		}

		var sb strings.Builder
		app.Git.SetWriter(&sb)

		path := filepath.Join(app.Path.Home(), name)
		path = files.EnsureExt(path, "db")
		if !files.Exists(path) {
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

		gr := gm.NewRepo(r.Name(),
			RepoFileReader(gm.Color()),
			RepoFileRemover(),
			RepoFileWriter(gm.Color()),
			RepoStatsReader(r),
		)

		if err := PruneRepo(ctx, gm, gr, bs); err != nil {
			if errors.Is(err, git.ErrGitUpToDate) {
				fmt.Fprintf(w, "git: repo %s up-to-date\n", p.BrightYellow.Sprint(name))
				continue
			}
			return err
		}

		fmt.Fprint(w, sb.String())
	}

	fmt.Fprintln(w, git.ErrGitUpToDate.Error())

	return nil
}

func StreamLog(ctx context.Context, g *git.Git) error {
	p := ansi.NewPalette(g.Color())
	logger := &git.LogStyle{
		Hash:         func(s string) string { return p.BrightYellow.Sprint(s) },
		Repo:         func(s string) string { return p.BrightGreen.Sprint(s) },
		Message:      func(s string) string { return s },
		Status:       func(s string) string { return p.Gray.With(p.Italic).Sprint(s) },
		Info:         func(s string) string { return p.BrightBlue.Sprint(s) },
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

func readFiles(color bool) func(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error) {
	sp := rotato.New(
		rotato.WithColor(color),
		rotato.WithMessage("starting..."),
		rotato.WithPrefixColor(rotato.StyleDim),
		rotato.WithSpinnerColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
		rotato.WithMessageColor(rotato.FgBrightBlue.With(rotato.StyleItalic)),
		rotato.WithFailSymbolColor(rotato.FgBrightRed.With(rotato.StyleBold)),
		rotato.WithFailMessageColor(rotato.FgBrightRed.With(rotato.StyleBold)),
	)

	return func(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error) {
		return newRepoReader(ctx, &RepoReaderCfg{
			name:     filepath.Base(path),
			root:     path,
			fullpath: path,
			total:    total,
			spinner:  sp,
		})
	}
}

func addFiles(color bool) func(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
	return func(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		sp := rotato.New(
			rotato.WithColor(color),
			rotato.WithMessage("starting..."),
			rotato.WithPrefix("Git Tracker"),
			rotato.WithPrefixColor(rotato.StyleDim),
			rotato.WithSpinnerColor(rotato.FgBrightYellow.With(rotato.StyleBold)),
			rotato.WithMessageColor(rotato.FgBrightBlue.With(rotato.StyleItalic)),
			rotato.WithFailSymbolColor(rotato.FgBrightRed.With(rotato.StyleBold)),
			rotato.WithFailMessageColor(rotato.FgBrightRed.With(rotato.StyleBold)),
		)

		sp.Start(ctx)
		defer sp.Done()

		root := filepath.Dir(repoPath)
		if gpg.IsInitialized(root) {
			return addGPGFiles(ctx, bs, sp, repoPath)
		}

		for i := range bs {
			if _, err := bookio.SaveAsJSON(repoPath, bs[i], true); err != nil {
				return err
			}
		}

		return nil
	}
}

func removeFiles(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c, err := bookio.NewFileRemover(repoPath, files.DefaultManager, genFullpath)
	if err != nil {
		return err
	}

	return c.Rm(ctx, bs)
}

func genFullpath(repoPath string, b *bookmark.Bookmark) (string, error) {
	var filename string
	var err error

	if gpg.IsInitialized(filepath.Dir(repoPath)) {
		filename, err = b.GPGPath()
		if err != nil {
			return "", err
		}
	} else {
		filename, err = b.JSONPath()
		if err != nil {
			return "", err
		}
	}

	// [[GOMARKS_HOME/git]/[repoName][domain/bookmark.ext]]
	fullpath := filepath.Join(repoPath, filename)

	return fullpath, nil
}

var actionRe = regexp.MustCompile(`^(add|update|del|edit|import|sync|http|wayback|commit|params|gpg)\b`)

// newHighlighter returns a function that closes over the pre-computed map.
func newHighlighter(p *ansi.Palette) func(string) string {
	replacements := map[string]string{
		"add":     p.BrightCyan.Sprint("add"),
		"update":  p.BrightBlue.Sprint("update"),
		"del":     p.BrightRed.Sprint("del"),
		"edit":    p.Orange.Sprint("edit"),
		"import":  p.BrightYellow.Sprint("import"),
		"sync":    p.Cyan.Sprint("sync"),
		"http":    p.Magenta.Sprint("http"),
		"wayback": p.Red.Sprint("wayback"),
		"commit":  p.BrightMagenta.Sprint("commit"),
		"params":  p.Blue.Sprint("params"),
		"gpg":     p.Red.Sprint("gpg"),
	}

	return func(msg string) string {
		return actionRe.ReplaceAllStringFunc(msg, func(v string) string {
			return replacements[v]
		})
	}
}
