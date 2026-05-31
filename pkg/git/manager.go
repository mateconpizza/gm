package git

import (
	"context"
	"errors"
	"fmt"
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
	root  string
	track *Tracker
	*MgrOptions
}

func (m *Mgr) Root() string                                         { return m.root }
func (m *Mgr) IsEnabled() bool                                      { return fileExists(m.root) }
func (m *Mgr) Git() *Git                                            { return m.g }
func (m *Mgr) Init(ctx context.Context, force bool) error           { return m.g.Init(ctx, force) }
func (m *Mgr) Summary(gr *Repo) (*Summary, error)                   { return gr.Summary() }
func (m *Mgr) IsTracked(name string) bool                           { return m.track.Contains(name) }
func (m *Mgr) Repos() []string                                      { return m.track.Repos() }
func (m *Mgr) WriteRepos() error                                    { return m.track.Write() }
func (m *Mgr) Track(names ...string) error                          { return m.track.Track(names...) }
func (m *Mgr) Drop(ctx context.Context, gr *Repo) error             { return dropRepo(ctx, m.g, gr) }
func (m *Mgr) SaveChanges(ctx context.Context, gc *CommitCfg) error { return saveChanges(ctx, m, gc) }
func (m *Mgr) Commit(ctx context.Context, msg string) error         { return commitIfChanged(ctx, m.g, msg) }
func (m *Mgr) SetCfg(ctx context.Context, k, v string) error        { return m.g.SetCfgLocal(ctx, k, v) }

func (m *Mgr) Untrack(ctx context.Context, gr *Repo, msg string) error {
	return untrackRemoveRepo(ctx, m, gr, msg)
}

func (m *Mgr) NewRepo(name string, opts ...RepoOptFunc) *Repo {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return NewRepo(name, filepath.Join(m.Root(), name), opts...)
}

func (m *Mgr) Update(ctx context.Context, gr *Repo, old, fresh *bookmark.Bookmark) error {
	if m.version == "" {
		return ErrNoVersionFound
	}

	if err := updateRepo(ctx, gr, old, fresh); err != nil {
		return err
	}

	return m.SaveChanges(ctx, NewCommitCfg(
		gr,
		m.version,
		fmt.Sprintf("[%s] update bookmark", gr.Name()),
	))
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
