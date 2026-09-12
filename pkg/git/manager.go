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
}

func WithGit(g *Git) MgrOptFunc {
	return func(mo *MgrOptions) {
		mo.g = g
	}
}

func WithVersion(ver string) MgrOptFunc {
	return func(mo *MgrOptions) {
		mo.version = ver
	}
}

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

	t := NewTracker(rootDir)
	if err := t.Load(); err != nil {
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
func (gm *Mgr) Git() *Git                                     { return gm.g }
func (gm *Mgr) Init(ctx context.Context, force bool) error    { return gm.g.Init(ctx, force) }
func (gm *Mgr) IsTracked(name string) bool                    { return gm.track.Contains(name) }
func (gm *Mgr) Repos() []string                               { return gm.track.Repos() }
func (gm *Mgr) WriteRepos() error                             { return gm.track.Write() }
func (gm *Mgr) Version() string                               { return gm.version }
func (gm *Mgr) Track(names ...string) error                   { return gm.track.Track(names...) }
func (gm *Mgr) Commit(ctx context.Context, msg string) error  { return gm.g.commitIfChanged(ctx, msg) }
func (gm *Mgr) SetCfg(ctx context.Context, k, v string) error { return gm.g.SetCfgLocal(ctx, k, v) }

func (gm *Mgr) SaveChanges(ctx context.Context, gr *Repo, msg string) error {
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

func (gm *Mgr) Update(ctx context.Context, gr *Repo, old, fresh *bookmark.Bookmark, postRm PostRemovalFunc) error {
	if gm.version == "" {
		return ErrNoVersionFound
	}
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

func (gm *Mgr) Drop(ctx context.Context, gr *Repo) error {
	keep := map[string]struct{}{
		SummaryFileName: {},
	}
	err := removeAllExcept(gr.fullpath, keep)
	if err != nil {
		return err
	}
	return gm.SaveChanges(ctx, gr, fmt.Sprintf("[%s] drop repo", gr.Name()))
}

func (gm *Mgr) Untrack(ctx context.Context, gr *Repo, msg string) error {
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

func (gm *Mgr) UpdateAndSave(ctx context.Context, gr *Repo, old, fresh *bookmark.Bookmark, postRm PostRemovalFunc) error {
	if gm.version == "" {
		return ErrNoVersionFound
	}
	if err := gm.Update(ctx, gr, old, fresh, postRm); err != nil {
		return err
	}
	return gm.SaveChanges(ctx, gr, fmt.Sprintf("[%s] update bookmark", gr.Name()))
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
