package git

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func commitIfChanged(ctx context.Context, g *Git, msg string) error {
	changed, err := g.HasChanges(ctx)
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}
	if !changed {
		return nil
	}

	if err := g.AddAll(ctx); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}

	status, err := g.Status(ctx)
	if err != nil {
		status = ""
	}

	if status != "" {
		status = " (" + status + ")"
	}

	if err := g.Commit(ctx, fmt.Sprintf("%s%s", strings.ToLower(msg), status)); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return nil
}

func untrackRemoveRepo(ctx context.Context, m *Mgr, gr *Repo, msg string) error {
	if !m.IsTracked(gr.Name()) {
		return fmt.Errorf("%w: %q", ErrGitNotTracked, gr.Name())
	}

	if err := m.track.Untrack(gr.Name()); err != nil {
		return err
	}

	if err := m.WriteRepos(); err != nil {
		return err
	}

	if err := os.RemoveAll(gr.Fullpath()); err != nil {
		return err
	}

	return commitIfChanged(ctx, m.Git(), msg)
}

func updateRepo(ctx context.Context, gr *Repo, old, fresh *bookmark.Bookmark) error {
	if gr.db == nil {
		return fmt.Errorf("%w: in repo %q", ErrNoStoreFound, gr.name)
	}

	if err := gr.Rm(ctx, old); err != nil {
		return err
	}

	return gr.Add(ctx, []*bookmark.Bookmark{fresh})
}

func saveChanges(ctx context.Context, m *Mgr, gc *CommitCfg) error {
	if gc.gr.DB() == nil {
		return fmt.Errorf("%w: stats loader", ErrNoFunctionFound)
	}

	var (
		db = gc.gr.DB()
		gr = gc.gr
	)

	oldStats, err := gr.Stats()
	if err != nil {
		return err
	}

	freshStats, err := gr.StatsFromDB(ctx, db)
	if err != nil {
		return err
	}

	changed, err := HasChanges(ctx, m.Root())
	if err != nil {
		return err
	}

	if oldStats.Equal(freshStats) && !changed {
		return ErrGitUpToDate
	}

	sum, err := summaryComplete(ctx, m.Git(), freshStats, gc.ver)
	if err != nil {
		return err
	}

	if err := sum.Validate(); err != nil {
		return err
	}

	if err := gr.WriteSummary(sum); err != nil {
		return err
	}

	return commitIfChanged(ctx, m.Git(), gc.msg)
}

func dropRepo(ctx context.Context, g *Git, gr *Repo) error {
	if err := os.RemoveAll(gr.Fullpath()); err != nil {
		return err
	}
	return commitIfChanged(ctx, g, fmt.Sprintf("[%s] remove tracking", gr.Name()))
}
