package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrNoVersionFound = errors.New("git: no mgr version found")

type MgrOptFunc func(*MgrOptions)

type MgrOptions struct {
	g       *Git
	version string
	color   bool
}

func WithGit(g *Git) MgrOptFunc         { return func(mo *MgrOptions) { mo.g = g } }
func WithVersion(ver string) MgrOptFunc { return func(mo *MgrOptions) { mo.version = ver } }
func WithColor(b bool) MgrOptFunc       { return func(mo *MgrOptions) { mo.color = b } }

type Mgr struct {
	*MgrOptions

	root  string
	track *Tracker
}

func NewManager(rootDir string, opts ...MgrOptFunc) (*Mgr, error) {
	o := &MgrOptions{}
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
		track:      t,
		MgrOptions: o,
	}, nil
}

func (gm *Mgr) Root() string                                  { return gm.root }
func (gm *Mgr) IsEnabled() bool                               { return fileExists(gm.root) }
func (gm *Mgr) Color() bool                                   { return gm.color }
func (gm *Mgr) Git() *Git                                     { return gm.g }
func (gm *Mgr) IsTracked(name string) bool                    { return gm.track.contains(name) }
func (gm *Mgr) Repos() []string                               { return gm.track.list() }
func (gm *Mgr) WriteRepos() error                             { return gm.track.write() }
func (gm *Mgr) Version() string                               { return gm.version }
func (gm *Mgr) Track(names ...string) error                   { return gm.track.track(names...) }
func (gm *Mgr) SetCfg(ctx context.Context, k, v string) error { return gm.g.SetCfgLocal(ctx, k, v) }

func (gm *Mgr) Commit(ctx context.Context, msg CommitMessage) error {
	return gm.g.commitIfChanged(ctx, msg)
}

func (gm *Mgr) Init(ctx context.Context, force bool) error {
	if force {
		gm.track.reset()
	}
	return gm.g.Init(ctx, force)
}

func (gm *Mgr) SaveChanges(ctx context.Context, gr *Repo, msg CommitMessage) error {
	if gm.version == "" {
		return ErrNoVersionFound
	}
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
	should, err := gm.shouldSave(ctx, oldStats, freshStats)
	if err != nil {
		return err
	}
	if !should {
		return ErrGitUpToDate
	}
	// FIX: update full summary only in git push.
	sum, err := summaryComplete(ctx, gm.g, freshStats, os.Hostname, gm.version)
	if err != nil {
		return err
	}
	if err := sum.Validate(); err != nil {
		return err
	}
	if err := gr.WriteSummary(sum); err != nil {
		return err
	}
	return gm.g.commitIfChanged(ctx, msg)
}

func (gm *Mgr) NewRepo(name string, opts ...RepoOptFunc) *Repo {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return NewRepo(name, filepath.Join(gm.Root(), name), opts...)
}

func (gm *Mgr) Update(ctx context.Context, p UpdateParams) error {
	if gm.version == "" {
		return ErrNoVersionFound
	}
	if p.Repo.db == nil {
		return fmt.Errorf("%w: in repo %q", ErrNoStoreFound, p.Repo.name)
	}
	if err := p.Repo.Rm(ctx, p.Old, p.PostRmFn); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", p.Old.URL, err)
		}
	}
	return p.Repo.Add(ctx, []*bookmark.Bookmark{p.Fresh})
}

func (gm *Mgr) Drop(ctx context.Context, gr *Repo) error {
	keep := map[string]struct{}{
		SummaryFileName: {},
	}
	err := removeAllExcept(gr.fullpath, keep)
	if err != nil {
		return err
	}
	return gm.SaveChanges(ctx, gr, gr.CommitMsg(Del, "repo"))
}

func (gm *Mgr) Untrack(ctx context.Context, gr *Repo) error {
	if !gm.IsTracked(gr.Name()) {
		return fmt.Errorf("%w: %q", ErrGitNotTracked, gr.Name())
	}
	if err := gm.track.untrack(gr.Name()); err != nil {
		return err
	}
	if err := gm.WriteRepos(); err != nil {
		return err
	}
	if err := os.RemoveAll(gr.Fullpath()); err != nil {
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

type UpdateParams struct {
	Repo     *Repo
	Old      *bookmark.Bookmark
	Fresh    *bookmark.Bookmark
	PostRmFn PostRemovalFunc
}

func (gm *Mgr) UpdateAndSave(ctx context.Context, p UpdateParams, msg CommitMessage) error {
	if err := gm.Update(ctx, p); err != nil {
		return err
	}
	return gm.SaveChanges(ctx, p.Repo, msg)
}

func (gm *Mgr) shouldSave(ctx context.Context, old, fresh *RepoStats) (bool, error) {
	changed, err := gm.g.HasChanges(ctx)
	if err != nil {
		return false, err
	}
	if !changed && old.Equal(fresh) {
		return false, nil
	}
	return true, nil
}
