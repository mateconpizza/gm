package gitops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
	"github.com/mateconpizza/rotato"
)

var _ bookio.FileManager = (*files.FileManager)(nil)

func RepoFileReader() git.RepoOptFunc              { return git.WithRepoReader(readFiles) }
func RepoFileWriter() git.RepoOptFunc              { return git.WithRepoWriter(addFiles) }
func RepoFileRemover() git.RepoOptFunc             { return git.WithRepoRemover(removeFiles) }
func RepoStatsReader(r *db.SQLite) git.RepoOptFunc { return git.WithRepoStore(r) }
func MgrVersion(ver string) git.MgrOptFunc         { return git.WithVersion(ver) }

func Init(ctx context.Context, app *application.App, m *git.Mgr) error {
	if err := m.Init(ctx, app.Flags.Reinit); err != nil {
		if errors.Is(err, git.ErrGitInitialized) {
			s := ansi.BrightYellow.With(ansi.Italic).Sprint("git init --reinit")
			return fmt.Errorf("%w, use %s", err, s)
		}
		return err
	}

	c := ui.NewDefaultConsole(ctx, func(err error) { sys.ErrAndExit(err) })
	if err := AskForEncryption(ctx, c, app, m); err != nil {
		return err
	}

	if err := c.Term().Print(ctx, c.SuccessMesg("git initialized\n")); err != nil {
		return err
	}

	app.Git.Enabled = true
	return app.WriteConfig(true)
}

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

func Sync(ctx context.Context, app *application.App, msg string) error {
	slog.Debug("starting git sync")
	if !app.GitEnabled() {
		slog.Warn("git sync: disabled")
		return nil
	}

	m, err := NewManager(app)
	if err != nil {
		return fmt.Errorf("git sync: failed to create git repo: %w", err)
	}

	if !m.IsEnabled() {
		slog.Debug("git sync disabled, skipping", "enabled", m.IsEnabled())
		return nil
	}

	if !m.IsTracked(app.DBBaseName()) {
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

	gr := NewRepo(m, r.Name(), git.WithRepoStore(r))
	if err := gr.Add(ctx, bs); err != nil {
		return fmt.Errorf("git sync: failed to add bookmarks: %w", err)
	}

	return m.SaveChanges(ctx, gr, msg)
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
