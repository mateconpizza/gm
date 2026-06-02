package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

type MkFileFunc func(ctx context.Context, b *bookmark.Bookmark) error

func RepoFileWriter() git.RepoOptFunc {
	return git.WithRepoWriter(addFile)
}

func RepoFileRemover() git.RepoOptFunc {
	return git.WithRepoRemover(removeFile)
}

func RepoStatsReader(r *db.SQLite) git.RepoOptFunc {
	return git.WithRepoStore(r)
}

func MgrVersion(ver string) git.MgrOptFunc {
	return git.WithVersion(ver)
}

func RepoFileReader() git.RepoOptFunc {
	return git.WithRepoReader(func(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error) {
		root := filepath.Dir(path)
		return NewRepoReader(ctx, root, path, total)
	})
}

func addFile(ctx context.Context, repoPath string, b *bookmark.Bookmark) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	fullpath, err := genFullpath(repoPath, b)
	if err != nil {
		return err
	}

	root := filepath.Dir(repoPath)
	if gpg.IsInitialized(root) {
		fingerprintPath := gpg.GPGIDPath(root)
		return createGPGFile(ctx, fullpath, fingerprintPath, b)
	}
	return createJSONFile(fullpath, b)
}

func removeFile(ctx context.Context, repoPath string, b *bookmark.Bookmark) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	fullpath, err := genFullpath(repoPath, b)
	if err != nil {
		return err
	}

	if !files.Exists(fullpath) {
		slog.Debug("remove bookmark: not found", "file", fullpath)
		return nil
	}

	return files.Remove(fullpath)
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
		slog.ErrorContext(ctx, "failed to create git repo", "error", err)
		return err
	}

	if !m.IsEnabled() {
		slog.DebugContext(ctx, "git sync disabled, skipping", "enabled", app.Git.Enabled)
		return nil
	}

	if !m.IsTracked(app.DBBaseName()) {
		slog.DebugContext(ctx, "database path not tracked in git, skipping sync")
		return nil
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		slog.ErrorContext(ctx, "failed to open database", "error", err)
		return err
	}
	defer r.Close()

	bs, err := r.All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch bookmarks", "error", err)
		return err
	}

	gr := m.NewRepo(r.BaseName(), RepoFileWriter())
	if err := gr.Add(ctx, bs); err != nil {
		return err
	}

	sum, err := getSummary(ctx, r, gr)
	if err != nil {
		return err
	}

	if err := gr.WriteSummary(sum); err != nil {
		return err
	}

	if err := m.Commit(ctx, msg); err != nil {
		slog.ErrorContext(ctx, "failed to commit changes", "error", err)
		return err
	}

	return nil
}

func createGPGFile(ctx context.Context, fullpath, fingerprintPath string, b *bookmark.Bookmark) error {
	if err := files.MkdirAll(filepath.Dir(fullpath)); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	data, err := json.MarshalIndent(b.JSON(), "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	return gpg.Encrypt(ctx, fingerprintPath, fullpath, data)
}

func createJSONFile(fullpath string, b *bookmark.Bookmark) error {
	if _, err := files.JSONWrite(fullpath, b.JSON(), true); err != nil {
		return err
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
