package gitops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
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

func RepoFileReader() git.RepoOptFunc              { return git.WithRepoReader(readFiles) }
func RepoFileWriter() git.RepoOptFunc              { return git.WithRepoWriter(addFiles) }
func RepoFileRemover() git.RepoOptFunc             { return git.WithRepoRemover(removeFiles) }
func RepoStatsReader(r *db.SQLite) git.RepoOptFunc { return git.WithRepoStore(r) }
func MgrVersion(ver string) git.MgrOptFunc         { return git.WithVersion(ver) }

// Init initializes Git support and configures repository encryption.
func Init(ctx context.Context, app *application.App, m *git.Mgr) error {
	if err := m.Init(ctx, app.Flags.Reinit); err != nil {
		if errors.Is(err, git.ErrGitInitialized) {
			s := ansi.BrightYellow.With(ansi.Italic).Sprint("git init --reinit")
			return fmt.Errorf("%w, use %s", err, s)
		}
		return err
	}

	c := ui.DefaultConsole
	if err := AskForEncryption(ctx, c, app, m); err != nil {
		return err
	}

	if err := c.Print(ctx, c.SuccessMesg("git initialized\n")); err != nil {
		return err
	}

	app.Git.Enabled = true
	return app.WriteConfig(true)
}

// Push pushes any unpushed commits to the configured upstream remote.
func Push(ctx context.Context, app *application.App, m *git.Mgr) error {
	g := m.Git()
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
func Sync(ctx context.Context, app *application.App, msg string) error {
	slog.Debug("starting git sync")
	if !app.GitEnabled() {
		slog.Warn("git sync: disabled")
		return nil
	}

	gm, err := NewManager(app)
	if err != nil {
		return fmt.Errorf("git sync: failed to create git repo: %w", err)
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

	gr := NewRepo(gm, r.Name(), RepoStatsReader(r))
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

	gm, err := NewManager(app)
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

		if err := pruneRepo(ctx, gm, r); err != nil {
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

func readFiles(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error) {
	root := filepath.Dir(path)
	return NewRepoReader(ctx, root, path, total)
}

func addFiles(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	sp := rotato.New(
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
