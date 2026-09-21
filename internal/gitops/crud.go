package gitops

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type store interface {
	Stats(ctx context.Context, dest any) error
	All(ctx context.Context) ([]*bookmark.Bookmark, error)
}

type console interface {
	Confirm(ctx context.Context, q, def string) bool
	SuccessMesg(a ...any) string
	Print(ctx context.Context, s string) error
}

type ManagerConfig struct {
	Root    string
	Writer  io.Writer
	Version string
	Color   bool
}

func (mc *ManagerConfig) Validate() error {
	if mc.Writer == nil {
		mc.Writer = os.Stdout
	}

	return nil
}

func NewManager(cfg *ManagerConfig) (*git.Mgr, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	g, err := NewGit(cfg.Writer, cfg.Root, cfg.Color)
	if err != nil {
		return nil, err
	}

	return git.NewManager(
		cfg.Root,
		git.WithGit(g),
		git.WithVersion(cfg.Version),
		git.WithColor(cfg.Color),
	)
}

func NewGit(w io.Writer, root string, color bool) (*git.Git, error) {
	p := ansi.NewPalette(color)
	return git.New(
		root,
		[]git.GitOpt{
			// command logger
			git.WithGitCmdLogger(func(w io.Writer, commands []string) {
				headerFrame := frame.New(
					frame.WithColorBorder(p.BrightYellow.Sprint),
					frame.WithBordersSmallBlock(),
					frame.WithWriter(w),
				)
				fullCmd := p.BrightYellow.Wrap(strings.Join(commands, " "), p.Italic)
				headerFrame.Midln(fullCmd).Flush()
			}),

			// writer
			git.WithGitWriter(w),
			// color
			git.WithGitColor(color),
		}...,
	)
}

func Add(ctx context.Context, gm gitManager, gr *git.Repo, b *bookmark.Bookmark) error {
	if !gm.IsEnabled() || !gm.IsTracked(gr.Name()) {
		return nil
	}
	if err := gr.Add(ctx, []*bookmark.Bookmark{b}); err != nil {
		return err
	}
	return gm.SaveChanges(ctx, gr, gr.CommitMsg(git.Add, "bookmark"))
}

func Remove(ctx context.Context, gm gitManager, gr *git.Repo, bs []*bookmark.Bookmark) error {
	if !gm.IsEnabled() || !gm.IsTracked(gr.Name()) {
		return nil
	}
	if err := gr.RmMany(ctx, bs, files.RemoveEmptyDirs); err != nil {
		return err
	}
	return gm.SaveChanges(ctx, gr, gr.CommitMsg(git.Del, func() string {
		if len(bs) > 1 {
			return "bookmarks"
		}
		return "bookmark"
	}()))
}

func Drop(ctx context.Context, gm gitManager, gr *git.Repo, c console) error {
	if !gm.IsEnabled() {
		slog.Debug("git repo: git disable")
		return nil
	}
	slog.Debug("git repo: start repo drop")
	if !gm.IsTracked(gr.Name()) {
		return nil
	}
	if !c.Confirm(ctx, "drop git repo?", "n") {
		return nil
	}
	if err := gm.Drop(ctx, gr); err != nil {
		return err
	}
	if !c.Confirm(ctx, "untrack database?", "n") {
		return nil
	}
	if err := gm.Untrack(ctx, gr); err != nil {
		return err
	}
	return c.Print(ctx, c.SuccessMesg("database untracked\n"))
}
