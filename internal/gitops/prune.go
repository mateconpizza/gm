package gitops

import (
	"context"
	"fmt"
	"log/slog"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type gitRepo interface {
	Name() string
	Read(ctx context.Context) error
	RmMany(ctx context.Context, bs []*bookmark.Bookmark, postRm git.PostRemovalFunc) error
	Bookmarks() []*bookmark.Bookmark
	Add(ctx context.Context, bs []*bookmark.Bookmark) error
}

type manager interface {
	Drop(ctx context.Context, gr *git.Repo) error
	Repos() []string
	Untrack(ctx context.Context, gr *git.Repo, msg string) error
	SaveChanges(ctx context.Context, gr *git.Repo, msg string) error
	IsTracked(name string) bool
	IsEnabled() bool
	UpdateAndSave(ctx context.Context, gr *git.Repo, old, fresh *bookmark.Bookmark, postRm git.PostRemovalFunc) error
}

type saveChangesFunc func(ctx context.Context, msg string) error

type RepoReconciler struct {
	repo           gitRepo
	dbBookmarks    []*bookmark.Bookmark
	persistChanges saveChangesFunc
}

func newRepoReconciler() *RepoReconciler {
	return &RepoReconciler{}
}

func (r *RepoReconciler) WithPersistFunc(fn saveChangesFunc) *RepoReconciler {
	r.persistChanges = fn
	return r
}

func (r *RepoReconciler) WithRepo(gr gitRepo) *RepoReconciler {
	r.repo = gr
	return r
}

func (r *RepoReconciler) WithBookmarks(bs []*bookmark.Bookmark) *RepoReconciler {
	r.dbBookmarks = bs
	return r
}

func (r *RepoReconciler) Reconcile(ctx context.Context) error {
	if err := r.readRepo(ctx); err != nil {
		return err
	}

	if err := r.addMissing(ctx); err != nil {
		return err
	}

	if err := r.pruneStale(ctx); err != nil {
		return err
	}

	if err := r.removeOrphans(ctx); err != nil {
		return err
	}

	return git.ErrGitUpToDate
}

func (r *RepoReconciler) readRepo(ctx context.Context) error {
	return r.repo.Read(ctx)
}

func (r *RepoReconciler) msg(msg string) string {
	return fmt.Sprintf("[%s] repo sync: %s", r.repo.Name(), msg)
}

// addMissing adds bookmarks missing from the repository.
func (r *RepoReconciler) addMissing(ctx context.Context) error {
	missing := bookmark.Difference(r.repo.Bookmarks(), r.dbBookmarks)

	if len(missing) == 0 {
		return nil
	}

	slog.Debug("git sync: found missing bookmarks", "count", len(missing))
	if err := r.repo.Add(ctx, missing); err != nil {
		return err
	}

	return r.persistChanges(ctx, r.msg("add missing"))
}

// pruneStale removes repository bookmarks not found in the database.
func (r *RepoReconciler) pruneStale(ctx context.Context) error {
	stale, _ := bookmark.Deduplicate(r.repo.Bookmarks(), r.dbBookmarks)

	if len(stale) == 0 {
		slog.Debug("git sync: no stale bookmarks found")
		return nil
	}

	slog.Debug("git sync: found stale bookmarks", "count", len(stale))
	if err := r.repo.RmMany(ctx, stale, files.RemoveEmptyDirs); err != nil {
		return err
	}

	return r.persistChanges(ctx, r.msg("prune stale"))
}

// removeOrphans removes orphaned bookmarks from the repository.
func (r *RepoReconciler) removeOrphans(ctx context.Context) error {
	diff := bookmark.Difference(r.dbBookmarks, r.repo.Bookmarks())

	if len(diff) == 0 {
		return nil
	}

	if err := r.repo.RmMany(ctx, diff, files.RemoveEmptyDirs); err != nil {
		return err
	}

	return r.persistChanges(ctx, r.msg("remove orphans"))
}

// PruneRepo runs the reconcile-and-persist cycle for a single repo.
func PruneRepo(ctx context.Context, gm manager, gr *git.Repo, bs []*bookmark.Bookmark) error {
	if !gm.IsTracked(gr.Name()) {
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, gr.Name())
	}

	persistFn := func(ctx context.Context, msg string) error {
		return gm.SaveChanges(ctx, gr, msg)
	}

	return newRepoReconciler().
		WithRepo(gr).
		WithBookmarks(bs).
		WithPersistFunc(persistFn).
		Reconcile(ctx)
}
