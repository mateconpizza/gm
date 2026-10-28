package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrNoVersionFound = errors.New("git: no mgr version found")

type (
	MgrOptFunc func(*MgrOptions)

	MgrOptions struct {
		g       *Git
		fileMgr *fileManager
	}
)

func WithGit(g *Git) MgrOptFunc { return func(mo *MgrOptions) { mo.g = g } }

type Mgr struct {
	*MgrOptions

	root    string
	tracker *Tracker
}

func NewManager(rootDir string, opts ...MgrOptFunc) (*Mgr, error) {
	o := &MgrOptions{
		fileMgr: newFileManager(),
	}
	for _, opt := range opts {
		opt(o)
	}

	t := newTracker(rootDir)
	if err := t.load(); err != nil {
		return nil, err
	}

	if o.g == nil {
		g, err := New(rootDir)
		if err != nil {
			return nil, err
		}
		o.g = g
	}

	return &Mgr{
		root:       rootDir,
		tracker:    t,
		MgrOptions: o,
	}, nil
}

func (gm *Mgr) Root() string                                  { return gm.root }
func (gm *Mgr) IsEnabled() bool                               { return fileExists(gm.root) }
func (gm *Mgr) Git() *Git                                     { return gm.g }
func (gm *Mgr) IsTracked(name string) bool                    { return gm.tracker.contains(name) }
func (gm *Mgr) Repos() []string                               { return gm.tracker.list() }
func (gm *Mgr) WriteRepos() error                             { return gm.tracker.write() }
func (gm *Mgr) Track(names ...string) error                   { return gm.tracker.track(names...) }
func (gm *Mgr) SetCfg(ctx context.Context, k, v string) error { return gm.g.SetCfgLocal(ctx, k, v) }

func (gm *Mgr) Commit(ctx context.Context, msg CommitMessage) error {
	return gm.g.commitIfChanged(ctx, msg)
}

func (gm *Mgr) CommitEmpty(ctx context.Context, msg CommitMessage) error {
	return gm.g.commitEmpty(ctx, msg)
}

func (gm *Mgr) Init(ctx context.Context, force bool) error {
	if force {
		gm.tracker.reset()
	}
	return gm.g.Init(ctx, force)
}

func (gm *Mgr) NewRepo(name string, opts ...RepoOptFunc) *Repo {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return NewRepo(name, filepath.Join(gm.Root(), name), opts...)
}

// Apply runs fn, which changes the repo's files, and then saves the changes.
func (gm *Mgr) Apply(ctx context.Context, msg CommitMessage, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}

	return gm.Commit(ctx, msg)
}

func (gm *Mgr) Drop(ctx context.Context, gr *Repo) error {
	keep := map[string]struct{}{
		SummaryFileName: {},
	}
	if err := gm.fileMgr.removeExcept(gr.fullpath, keep); err != nil {
		return err
	}
	return gm.Commit(ctx, gr.CommitMsg(Del, "repo"))
}

func (gm *Mgr) Untrack(ctx context.Context, gr *Repo) error {
	if !gm.IsTracked(gr.Name()) {
		return fmt.Errorf("%w: %q", ErrGitNotTracked, gr.Name())
	}
	if err := gm.tracker.untrack(gr.Name()); err != nil {
		return err
	}
	if err := gm.WriteRepos(); err != nil {
		return err
	}
	if err := gm.fileMgr.remove(gr.Fullpath()); err != nil {
		return err
	}
	return gm.Commit(ctx, gr.CommitMsg(Del, "tracking"))
}

// Push pushes any unpushed commits to the configured upstream remote.
func (gm *Mgr) Push(ctx context.Context) error {
	g := gm.Git()
	remote, err := g.Remote(ctx)
	if err != nil || remote == "" {
		return ErrGitNoUpstream
	}

	if err := g.SetUpstream(ctx, gm.root); err != nil {
		if !errors.Is(err, ErrGitUpstreamExists) {
			return err
		}
	}

	// check if there are unpushed commits
	should, err := g.HasUnpushedCommits(ctx)
	if err != nil {
		return err
	}
	if !should {
		return ErrGitUpToDate
	}

	return g.Push(ctx)
}

func (gm *Mgr) Rename(ctx context.Context, oldName, newName string) error {
	strip := func(s string) string { return strings.TrimSuffix(s, filepath.Ext(s)) }

	oldBase := strip(oldName)
	newBase := strip(newName)

	if oldBase == "" || newBase == "" {
		return fmt.Errorf("%w: name cannot be empty: old: %q, new: %q", os.ErrInvalid, oldBase, newBase)
	}

	if oldBase == newBase || !gm.IsTracked(oldBase) {
		return nil
	}

	src := filepath.Join(gm.root, oldBase)
	dest := filepath.Join(gm.root, newBase)

	if err := gm.fileMgr.rename(src, dest); err != nil {
		return fmt.Errorf(
			"failed to move git repo from %q to %q: %w",
			oldBase, newBase, err,
		)
	}

	// untrack oldRepo and track newRepo
	if err := gm.tracker.untrack(oldBase); err != nil {
		return err
	}
	if err := gm.tracker.track(newBase); err != nil {
		return err
	}
	if err := gm.WriteRepos(); err != nil {
		return fmt.Errorf("failed to update tracker after rename: %w", err)
	}

	gr := gm.NewRepo(newBase)

	sum, err := gr.Summary()
	if err != nil {
		return err
	}
	if err := gr.WriteSummary(sum); err != nil {
		return err
	}

	return gm.Commit(ctx, gr.CommitMsg(Rename, oldBase))
}
