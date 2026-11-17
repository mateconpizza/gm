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
	Fullpath() string
	CommitMsg(a git.RepoAction, obj string) git.CommitMessage
}

type gitManager interface {
	Drop(ctx context.Context, gr *git.Repo) error
	Repos() []string
	Untrack(ctx context.Context, gr *git.Repo) error
	IsTracked(name string) bool
	IsEnabled() bool
	Commit(ctx context.Context, msg git.CommitMessage) error
}

type saveChangesFunc func(ctx context.Context, msg git.CommitMessage) error

type RepoReconciler struct {
	repo           gitRepo
	dbBookmarks    []*bookmark.Bookmark
	persistChanges saveChangesFunc
	logger         *slog.Logger
}

func newRepoReconciler() *RepoReconciler {
	return &RepoReconciler{logger: slog.New(slog.DiscardHandler)}
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

func (r *RepoReconciler) WithLogger(l *slog.Logger) *RepoReconciler {
	if l != nil {
		r.logger = l.With("cmp", "reconciler")
	}
	return r
}

func (r *RepoReconciler) Reconcile(ctx context.Context) error {
	log := r.logger.With("fn", "reconcile")

	log.Debug("starting repository reconciliation",
		"db_bookmarks", len(r.dbBookmarks),
	)

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

	log.Debug("repository reconciliation completed")

	return git.ErrGitUpToDate
}

func (r *RepoReconciler) readRepo(ctx context.Context) error {
	log := r.logger.With("stage", "read-repo")

	log.Debug("reading repository bookmarks")

	if err := r.repo.Read(ctx); err != nil {
		log.Error("failed to read repository bookmarks", "error", err)
		return err
	}

	log.Debug("repository bookmarks loaded",
		"count", len(r.repo.Bookmarks()),
	)

	return nil
}

func (r *RepoReconciler) msg(action git.RepoAction, msg string) git.CommitMessage {
	return r.repo.CommitMsg(action, msg)
}

// addMissing adds bookmarks missing from the repository.
func (r *RepoReconciler) addMissing(ctx context.Context) error {
	log := r.logger.With("stage", "add-missing")

	missing := bookmark.Difference(r.repo.Bookmarks(), r.dbBookmarks)

	if len(missing) == 0 {
		log.Debug("no missing bookmarks found")
		return nil
	}

	log.Debug("adding missing bookmarks", "count", len(missing))

	if err := r.repo.Add(ctx, missing); err != nil {
		log.Error("failed to add missing bookmarks",
			"count", len(missing),
			"error", err,
		)
		return err
	}

	if err := r.persistChanges(ctx, r.msg(git.Add, "missing")); err != nil {
		log.Error("failed to commit added bookmarks", "error", err)
		return err
	}

	log.Debug("missing bookmarks added", "count", len(missing))

	return nil
}

// pruneStale removes repository bookmarks not found in the database.
func (r *RepoReconciler) pruneStale(ctx context.Context) error {
	log := r.logger.With("stage", "prune-stale")

	stale, _ := bookmark.Deduplicate(r.repo.Bookmarks(), r.dbBookmarks)

	if len(stale) == 0 {
		log.Debug("no stale bookmarks found")
		return nil
	}

	log.Debug("removing stale bookmarks", "count", len(stale))

	if err := r.repo.RmMany(ctx, stale, files.RemoveEmptyDirs); err != nil {
		log.Error("failed to remove stale bookmarks",
			"count", len(stale),
			"error", err,
		)
		return err
	}

	if err := r.persistChanges(ctx, r.msg(git.Del, "stale")); err != nil {
		log.Error("failed to commit removed bookmarks", "error", err)
		return err
	}

	log.Debug("stale bookmarks removed", "count", len(stale))

	return nil
}

// removeOrphans removes orphaned bookmarks from the repository.
func (r *RepoReconciler) removeOrphans(ctx context.Context) error {
	log := r.logger.With("stage", "remove-orphans")

	diff := bookmark.Difference(r.dbBookmarks, r.repo.Bookmarks())

	if len(diff) == 0 {
		log.Debug("no orphaned bookmarks found")
		return nil
	}

	log.Debug("removing orphaned bookmarks", "count", len(diff))

	if err := r.repo.RmMany(ctx, diff, files.RemoveEmptyDirs); err != nil {
		log.Error("failed to remove orphaned bookmarks",
			"count", len(diff),
			"error", err,
		)
		return err
	}

	if err := r.persistChanges(ctx, r.msg(git.Del, "orphans")); err != nil {
		log.Error("failed to commit removed orphaned bookmarks", "error", err)
		return err
	}

	log.Debug("orphaned bookmarks removed", "count", len(diff))

	return nil
}

// PruneRepo runs the reconcile-and-persist cycle for a single repo.
func PruneRepo(ctx context.Context, gm gitManager, gr *git.Repo, bs []*bookmark.Bookmark, log *slog.Logger) error {
	log = log.With("repo", gr.Name())
	plog := log.With("fn", "prune-repo")

	if !gm.IsTracked(gr.Name()) {
		plog.Debug("skipping untracked repository")
		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, gr.Name())
	}

	plog.Debug("pruning repository", "bookmarks", len(bs))

	persistFn := func(ctx context.Context, msg git.CommitMessage) error {
		return gm.Commit(ctx, msg)
	}

	return newRepoReconciler().
		WithRepo(gr).
		WithBookmarks(bs).
		WithPersistFunc(persistFn).
		WithLogger(log).
		Reconcile(ctx)
}
