package gitops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

type store interface {
	BaseName() string
	Stats(ctx context.Context, dest any) error
	All(ctx context.Context) ([]*bookmark.Bookmark, error)
	Close()
}

type console interface {
	Confirm(ctx context.Context, q, def string) bool
	SuccessMesg(a ...any) string
	Print(ctx context.Context, s string) error
	Choose(ctx context.Context, q string, opts []string, def string) (string, error)
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

func NewManager(app *application.App, opts ...git.GitOpt) (*git.Mgr, error) {
	w := app.Git.Writer()
	if w == nil {
		w = os.Stdout
	}

	root := app.Path.Git()

	opts = append(opts,
		NewGitCommandLogger(app.Flags.Color),
		git.WithLogger(app.Logger()),
		git.WithWriter(w),
	)
	g, err := git.New(root, opts...)
	if err != nil {
		return nil, err
	}

	gm, err := git.NewManager(root, git.WithGit(g))
	if err != nil {
		return nil, err
	}

	return gm, nil
}

func NewGitCommandLogger(color bool) git.GitOpt {
	p := ansi.NewPalette(color)

	// command logger
	return git.WithCmdLogger(func(w io.Writer, commands []string) {
		c := p.BrightYellow.With(p.Italic)
		headerFrame := frame.New(
			frame.WithColorBorder(c.Sprint),
			frame.WithBordersSmallBlock(),
			frame.WithWriter(w),
		)
		commands[0] = filepath.Base(commands[0])
		fullCmd := c.Sprint(strings.Join(commands, " "))
		headerFrame.Midln(fullCmd).Flush()
	})
}

func Add(ctx context.Context, gm gitManager, gr *git.Repo, b *bookmark.Bookmark) error {
	if !gm.IsEnabled() || !gm.IsTracked(gr.Name()) {
		return nil
	}
	if err := gr.Add(ctx, []*bookmark.Bookmark{b}); err != nil {
		return err
	}
	return gm.Commit(ctx, gr.CommitMsg(git.Add, "bookmark"))
}

func Remove(ctx context.Context, gm gitManager, gr *git.Repo, bs []*bookmark.Bookmark) error {
	if !gm.IsEnabled() || !gm.IsTracked(gr.Name()) {
		return nil
	}
	if err := gr.RmMany(ctx, bs, files.RemoveEmptyDirs); err != nil {
		return err
	}
	return gm.Commit(ctx, gr.CommitMsg(git.Del, func() string {
		if len(bs) > 1 {
			return "bookmarks"
		}
		return "bookmark"
	}()))
}

func Drop(ctx context.Context, gm gitManager, gr *git.Repo, c console, log *slog.Logger) error {
	log = log.With("op", "git.drop", "repo", gr.Name())

	if !gm.IsEnabled() {
		log.Debug("git disabled, skipping")
		return nil
	}
	if !gm.IsTracked(gr.Name()) {
		log.Debug("repo not tracked, skipping")
		return nil
	}

	if !c.Confirm(ctx, "drop git repo?", "n") {
		log.Debug("drop declined by user")
		return nil
	}
	if err := gm.Drop(ctx, gr); err != nil {
		if errors.Is(err, git.ErrGitUpToDate) {
			log.Debug("drop failed", "err", git.ErrGitUpToDate)
			return nil
		}
		log.Error("drop failed", "err", err)
		return fmt.Errorf("drop git repo %q: %w", gr.Name(), err)
	}
	log.Info("git repo dropped")

	if !c.Confirm(ctx, "untrack database?", "n") {
		log.Debug("untrack declined by user")
		return nil
	}
	if err := gm.Untrack(ctx, gr); err != nil {
		log.Error("untrack failed", "err", err)
		return fmt.Errorf("untrack %q: %w", gr.Name(), err)
	}
	log.Info("repo untracked")

	return c.Print(ctx, c.SuccessMesg("database untracked\n"))
}
