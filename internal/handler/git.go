package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

func GitClone(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	// keep temp path lifecycle here so the defer cleanly wipes it out at the
	// very end
	tmpPath := filepath.Join(os.TempDir(), app.Name+"-clone")
	if files.Exists(tmpPath) {
		_ = files.RemoveAll(tmpPath)
	}
	fn := func() { _ = files.RemoveAll(tmpPath) }
	defer fn()

	t := d.Console().Term()
	t.SetInterruptFn(func(err error) {
		fn()
		sys.ErrAndExit(err)
	})

	gp, err := fetchGitRepos(ctx, d, app, tmpPath)
	if err != nil {
		return err
	}

	for _, gr := range gp.Repos() {
		if gpg.IsInitialized(gr.Root()) &&
			!t.Confirm(fmt.Sprintf("read encrypted repository %q?", gr.Name()), "yes") {
			continue
		}

		if err := processRepo(ctx, d, gp, gr); err != nil {
			return err
		}
	}

	return nil
}

func fetchGitRepos(ctx context.Context, d *deps.Deps, app *application.App, tmpPath string) (*gitops.GitPuller, error) {
	g, err := gitops.NewGit(app)
	if err != nil {
		return nil, err
	}

	if err := g.CloneInto(ctx, app.Git.Remote, tmpPath); err != nil {
		return nil, fmt.Errorf("cloning remote repo: %w", err)
	}

	gp := gitops.NewPuller(
		d.Console(),
		tmpPath,
		g.Root(),
	)
	if err := gp.Pull(); err != nil {
		return nil, err
	}

	p := picker.New[*git.Repo](
		app,
		menu.WithHeader("select repo/s"),
		menu.WithArgs("--cycle"),
		menu.WithHeaderLabel(" importing from git "),
		menu.WithHeader("select record/s to import"),
		menu.WithInterruptFn(d.Console().Term().InterruptFn),
		menu.WithMultiSelection(),
	)

	err = gp.Select(p, ui.NewDefaultConsole(ctx, func(err error) {
		fmt.Println(err.Error())
	}))
	if err != nil {
		return nil, err
	}

	return gp, nil
}

func processRepo(ctx context.Context, d *deps.Deps, gp *gitops.GitPuller, gr *git.Repo) error {
	if err := gp.Read(ctx); err != nil {
		return err
	}

	if err := gp.PrintDetails(gr); err != nil {
		if errors.Is(err, git.ErrGitRepoEmpty) {
			return nil // move to next
		}
		return err
	}

	return handleImportLoop(ctx, d, gr)
}

func handleImportLoop(ctx context.Context, d *deps.Deps, gr *git.Repo) error {
	var (
		c    = d.Console()
		p    = c.Palette()
		opts = []string{"merge", "create", "select"}
		bs   = gr.Bookmarks()
	)

	for {
		opt, err := c.Choose(p.Bold.Wrap(gr.Name(), p.Italic)+": import mode?", opts, "m")
		if err != nil {
			return err
		}

		switch opt {
		case "m":
			return insertRecords(ctx, d, bs)

		case "c":
			return handleCreateRepoMode(ctx, d, gr, bs)

		case "s":
			app, err := d.Application(ctx)
			if err != nil {
				return err
			}

			m := picker.New[*bookmark.Bookmark](
				app,
				menu.WithNth("3.."),
				menu.WithMultiSelection(),
			)
			m.SetFormatter(func(b **bookmark.Bookmark) string { return formatter.OnelineFunc(c, *b) })

			bs, err = m.Select(bs)
			if err != nil {
				return err
			}

			if idx := slices.Index(opts, "select"); idx != -1 {
				opts = slices.Delete(opts, idx, len(opts))
			}
		}
	}
}

func handleCreateRepoMode(ctx context.Context, d *deps.Deps, gr *git.Repo, bs []*bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	p := filepath.Join(app.Path.Data, gr.Name())
	p = files.EnsureSuffix(p, ".db")

	if files.Exists(p) {
		c := d.Console()
		p = renameRepo(p)
		c.Warning(fmt.Sprintf("%q repo already exists\n", gr.Name())).
			Info(fmt.Sprintf("renamed to %q\n", filepath.Base(p))).
			Rowln().
			Flush()
	}

	return createRepo(ctx, d, p, bs)
}

func insertRecords(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}

	bs, err = port.DeduplicateReport(ctx, d.Console(), r, bs)
	if err != nil {
		return err
	}

	c := d.Console()
	if len(bs) == 0 {
		_ = c.Term().Print(ctx, c.Warning("nothing to import\n").StringReset())
		return nil
	}

	if err := r.InsertMany(ctx, bs); err != nil {
		return err
	}

	return c.Term().Print(
		ctx,
		c.SuccessMesg("inserted ", len(bs), " into ", r.Name()+"\n"),
	)
}

func createRepo(ctx context.Context, d *deps.Deps, repoPath string, bs []*bookmark.Bookmark) error {
	r, err := db.Init(ctx, repoPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := r.InsertMany(ctx, bs); err != nil {
		return err
	}

	defer func() {
		c := d.Console()
		c.Frame().Reset().
			Success("Initialized database: " + c.Palette().Italic.Sprint(r.Name()) + "\n").Flush()
	}()

	return MigrationsStatus(ctx, d)
}

func renameRepo(path string) string {
	t := time.Now().Format(txt.TimeLayout)
	base := files.StripSuffixes(path)
	root := filepath.Dir(base)
	name := files.StripSuffixes(filepath.Base(path))

	return files.EnsureSuffix(
		filepath.Join(root, name+"-"+t),
		".db",
	)
}
