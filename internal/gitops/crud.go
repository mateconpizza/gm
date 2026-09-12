package gitops

import (
	"context"
	"fmt"
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

	g, err := NewGit(cfg.Writer, cfg.Root)
	if err != nil {
		return nil, err
	}

	return git.NewManager(
		cfg.Root,
		git.WithGit(g),
		git.WithVersion(cfg.Version),
	)
}

func NewGit(w io.Writer, root string) (*git.Git, error) {
	return git.New(
		root,
		[]git.GitOpt{
			// add Command logger
			git.WithGitCommandLogger(func(w io.Writer, commands []string) {
				headerFrame := frame.New(
					frame.WithColorBorder(ansi.BrightYellow),
					frame.WithBordersSmallBlock(),
					frame.WithWriter(w),
				)
				fullCmd := ansi.BrightYellow.Wrap(strings.Join(commands, " "), ansi.Italic)
				headerFrame.Midln(fullCmd).Flush()
			}),

			// writer
			git.WithGitWriter(w),
		}...,
	)
}

func Add(ctx context.Context, gm manager, gr *git.Repo, b *bookmark.Bookmark) error {
	if !gm.IsEnabled() || !gm.IsTracked(gr.Name()) {
		return nil
	}

	if err := gr.Add(ctx, []*bookmark.Bookmark{b}); err != nil {
		return err
	}

	return gm.SaveChanges(ctx, gr, fmt.Sprintf("[%s] bookmark added", gr.Name()))
}

func Remove(ctx context.Context, gm manager, gr *git.Repo, bs []*bookmark.Bookmark) error {
	if !gm.IsEnabled() || !gm.IsTracked(gr.Name()) {
		return nil
	}

	if err := gr.RmMany(ctx, bs, files.RemoveEmptyDirs); err != nil {
		return err
	}

	return gm.SaveChanges(ctx, gr, fmt.Sprintf("[%s] remove bookmarks", gr.Name()))
}

func Drop(ctx context.Context, gm manager, gr *git.Repo, c console) error {
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

	if err := gm.Untrack(ctx, gr, fmt.Sprintf("[%s] remove tracking", gr.Name())); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("database untracked\n"))
}

func Update(ctx context.Context, gm manager, gr *git.Repo, old, fresh *bookmark.Bookmark) error {
	if !gm.IsEnabled() || !gm.IsTracked(gr.Name()) {
		return nil
	}

	return gm.UpdateAndSave(ctx, gr, old, fresh, files.RemoveEmptyDirs)
}
