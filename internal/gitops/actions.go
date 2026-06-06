package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

var _ bookio.FileManager = (*files.FileManager)(nil)

func RepoFileWriter() git.RepoOptFunc              { return git.WithRepoWriter(addFiles) }
func RepoFileRemover() git.RepoOptFunc             { return git.WithRepoRemover(removeFiles) }
func RepoStatsReader(r *db.SQLite) git.RepoOptFunc { return git.WithRepoStore(r) }
func MgrVersion(ver string) git.MgrOptFunc         { return git.WithVersion(ver) }

func RepoFileReader() git.RepoOptFunc {
	return git.WithRepoReader(func(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error) {
		root := filepath.Dir(path)
		return NewRepoReader(ctx, root, path, total)
	})
}

func addFiles(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	root := filepath.Dir(repoPath)
	if gpg.IsInitialized(root) {
		fingerprintPath := gpg.GPGIDPath(root)

		fp, err := gpg.LookupKey(fingerprintPath)
		if err != nil {
			return fmt.Errorf("gpg strategy: %w", err)
		}

		if err := fp.Validate(); err != nil {
			return err
		}

		g, err := gpg.New(fp.Fingerprint)
		if err != nil {
			return err
		}

		for i := range bs {
			if err := createGPGFile(ctx, g, repoPath, bs[i]); err != nil {
				return err
			}
		}

		return nil
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

	g, err := NewGit(app)
	if err != nil {
		return err
	}

	m, err := git.NewManager(app.Path.Git(), git.WithGit(g))
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

	gr := m.NewRepo(r.BaseName(), RepoFileWriter())
	if err := gr.Add(ctx, bs); err != nil {
		return fmt.Errorf("git sync: failed to add bookmarks: %w", err)
	}

	sum, err := getSummary(ctx, r, gr)
	if err != nil {
		return fmt.Errorf("git sync: failed to get summary: %w", err)
	}

	if err := gr.WriteSummary(sum); err != nil {
		return fmt.Errorf("git sync: failed to write summary: %w", err)
	}

	if err := m.Commit(ctx, msg); err != nil {
		return fmt.Errorf("git sync: failed to commit changes: %w", err)
	}

	return nil
}

func createGPGFile(ctx context.Context, g *gpg.GPG, repoPath string, b *bookmark.Bookmark) error {
	fullpath, err := genFullpath(repoPath, b)
	if err != nil {
		return fmt.Errorf("gpgfile: %w", err)
	}

	if files.Exists(fullpath) {
		slog.Warn("gpgfile: not found", "file", fullpath)
		return nil
	}

	if err := files.MkdirAll(filepath.Dir(fullpath)); err != nil {
		return fmt.Errorf("gpgfile: failed creating dir: %w, %q", err, filepath.Dir(fullpath))
	}

	data, err := json.MarshalIndent(b.JSON(), "", "  ")
	if err != nil {
		return fmt.Errorf("gpgfile: JSON marshal: %w", err)
	}

	if err := g.Encrypt(ctx, fullpath, data); err != nil {
		return fmt.Errorf("gpgfile: creating file: %w", err)
	}

	return nil
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
