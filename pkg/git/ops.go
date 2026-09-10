package git

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func untrackRemoveRepo(ctx context.Context, gm *Mgr, gr *Repo, msg string) error {
	if !gm.IsTracked(gr.Name()) {
		return fmt.Errorf("%w: %q", ErrGitNotTracked, gr.Name())
	}

	if err := gm.track.Untrack(gr.Name()); err != nil {
		return err
	}

	if err := gm.WriteRepos(); err != nil {
		return err
	}

	if err := os.RemoveAll(gr.Fullpath()); err != nil {
		return err
	}

	return gm.Commit(ctx, msg)
}

func updateRepo(ctx context.Context, gr *Repo, old, fresh *bookmark.Bookmark, postRm PostRemovalFunc) error {
	if gr.db == nil {
		return fmt.Errorf("%w: in repo %q", ErrNoStoreFound, gr.name)
	}

	if err := gr.Rm(ctx, old, postRm); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", old.URL, err)
		}
	}

	return gr.Add(ctx, []*bookmark.Bookmark{fresh})
}

func saveChanges(ctx context.Context, gm *Mgr, gr *Repo, ver, msg string) error {
	if gr.DB() == nil {
		return fmt.Errorf("%w: stats loader", ErrNoFunctionFound)
	}

	oldStats, err := gr.Stats()
	if err != nil {
		return err
	}

	freshStats, err := gr.StatsFromDB(ctx, gr.db)
	if err != nil {
		return err
	}

	g := gm.Git()
	changed, err := g.HasChanges(ctx)
	if err != nil {
		return err
	}

	if !changed && oldStats.Equal(freshStats) {
		return ErrGitUpToDate
	}

	// FIX: update full summary only in git push.
	sum, err := summaryComplete(ctx, g, freshStats, ver)
	if err != nil {
		return err
	}

	if err := sum.Validate(); err != nil {
		return err
	}

	if err := gr.WriteSummary(sum); err != nil {
		return err
	}

	return g.commitIfChanged(ctx, msg)
}

func dropRepo(ctx context.Context, gm *Mgr, gr *Repo) error {
	keep := map[string]struct{}{
		SummaryFileName: {},
	}

	err := removeAllExcept(gr.fullpath, keep)
	if err != nil {
		return err
	}

	return gm.SaveChanges(ctx, gr, fmt.Sprintf("[%s] drop repo", gr.Name()))
}
