package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

func GitClone(d *deps.Deps) error {
	app, err := d.Application()
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

	pu, err := fetchGitRepos(d, app, tmpPath)
	if err != nil {
		return err
	}

	for _, repo := range pu.Repos() {
		q := fmt.Sprintf("read encrypted repository %q?", repo.Name())
		if pu.IsGPG && !t.Confirm(q, "yes") {
			continue
		}

		if err := processRepo(d, app, pu, repo); err != nil {
			return err
		}
	}

	return nil
}

func fetchGitRepos(d *deps.Deps, app *application.App, tmpPath string) (*git.Puller, error) {
	g, err := git.New(d.Context(), tmpPath)
	if err != nil {
		return nil, err
	}

	if err := g.CloneInto(app.Git.Remote, tmpPath); err != nil {
		return nil, fmt.Errorf("cloning remote repo: %w", err)
	}

	pu := git.NewPuller(
		d.Console(),
		g.FullPath(),
		tmpPath,
		git.WithPullerEncrypted(gpg.IsInitialized(g.FullPath())),
	)
	if err := pu.Pull(); err != nil {
		return nil, err
	}

	if err := pu.Select(picker.New[*git.Repo](
		app,
		menu.WithHeader("select repo/s"),
		menu.WithArgs("--cycle"),
		menu.WithHeaderLabel(" importing from git "),
		menu.WithHeader("select record/s to import"),
		menu.WithInterruptFn(d.Console().Term().InterruptFn),
		menu.WithMultiSelection(),
	)); err != nil {
		return nil, err
	}

	return pu, nil
}

func processRepo(d *deps.Deps, app *application.App, pu *git.Puller, repo *git.Repo) error {
	if err := pu.Read(d.Context()); err != nil {
		return err
	}

	if err := pu.PrintDetails(repo); err != nil {
		if errors.Is(err, git.ErrGitRepoEmpty) {
			return nil // move to next
		}
		return err
	}

	return handleImportLoop(d, app, repo)
}

func handleImportLoop(d *deps.Deps, app *application.App, repo *git.Repo) error {
	var (
		c    = d.Console()
		p    = c.Palette()
		opts = []string{"merge", "create", "select"}
		bs   = repo.Bookmarks()
	)

	for {
		opt, err := c.Choose(p.Bold.Wrap(repo.Name(), p.Italic)+": import mode?", opts, "m")
		if err != nil {
			return err
		}

		switch opt {
		case "m":
			return insertRecords(d, bs)

		case "c":
			return handleCreateRepoMode(d, repo, bs)

		case "s":
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

func handleCreateRepoMode(d *deps.Deps, repo *git.Repo, bs []*bookmark.Bookmark) error {
	app, err := d.Application()
	if err != nil {
		return err
	}

	p := filepath.Join(app.Path.Data, repo.Name())
	p = files.EnsureSuffix(p, ".db")

	if files.Exists(p) {
		c := d.Console()
		p = renameRepo(p)
		c.Warning(fmt.Sprintf("%q repo already exists\n", repo.Name())).
			Info(fmt.Sprintf("renamed to %q\n", filepath.Base(p))).
			Rowln().
			Flush()
	}

	return createRepo(d, p, bs)
}

func insertRecords(d *deps.Deps, bs []*bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}

	ctx := d.Context()
	bs, err = port.DeduplicateReport(ctx, d.Console(), r, bs)
	if err != nil {
		return err
	}

	c := d.Console()
	if len(bs) == 0 {
		_ = c.Term().Print(ctx, c.Warning("nothing to import").Ln().StringReset())
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

func createRepo(d *deps.Deps, repoPath string, bs []*bookmark.Bookmark) error {
	ctx := d.Context()
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

	return MigrationsStatus(d)
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

func GitPrune(d *deps.Deps) error {
	slog.Debug("git prune: starting repository prune")
	app, err := d.Application()
	if err != nil {
		return err
	}

	if !app.Git.Enabled {
		slog.Debug("git prune: git not enabled")
		return nil
	}

	name := files.StripSuffixes(app.DBName)
	m := git.NewRepo(
		name,
		filepath.Join(app.Git.Path, name),
	)

	if err := m.Read(d.Context()); err != nil {
		return err
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}

	inDB, err := r.All(d.Context())
	if err != nil {
		return err
	}

	inRepo := m.Bookmarks()
	stale, _ := port.Deduplicate(inRepo, inDB)
	if len(stale) == 0 {
		slog.Debug("git sync: nothing found")
		return nil
	}

	return git.RemoveBookmarks(app, stale)
}
