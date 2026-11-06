package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

func EnableReadme(ctx context.Context, d *deps.Deps) error {
	return setReadme(ctx, d, true)
}

func DisableReadme(ctx context.Context, d *deps.Deps) error {
	return setReadme(ctx, d, false)
}

// setReadme updates the README configuration and synchronizes Git
// repositories.
func setReadme(ctx context.Context, d *deps.Deps, enabled bool) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	app.Git.Readme.Enabled = enabled

	gm := app.Git.Manager()
	g := gm.Git()

	update := removeReadmes
	msg := "del readme"

	if enabled {
		update = generateReadmes
		msg = "add readme"
	}

	if err := update(ctx, app, app.Databases()); err != nil {
		return err
	}

	if err := HookHasChanges().Run(ctx, g); err != nil {
		return err
	}

	if err := app.WriteConfig(true); err != nil {
		return err
	}

	if err := gm.Commit(ctx, CoreCommitMsg(msg)); err != nil {
		return err
	}

	return d.Console().Print(ctx, d.Console().SuccessMesg("settings saved\n"))
}

func generateReadmes(ctx context.Context, app *application.App, dbs []string) error {
	gm := app.Git.Manager()

	files.PrioritizeFile(dbs, app.DBName)
	for _, path := range dbs {
		if err := generateReadme(ctx, app, gm, path); err != nil {
			return err
		}
	}

	return nil
}

func generateReadme(ctx context.Context, app *application.App, gm *git.Mgr, path string) error {
	defaultDB := app.DBName
	defer func() {
		app.DBName = defaultDB
	}()

	r, err := db.New(ctx, path)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := app.SetDatabase(r.Name()); err != nil {
		return err
	}

	if !gm.IsEnabled() || !gm.IsTracked(app.DBBaseName()) {
		return nil
	}

	hook := HookReadme(
		ReadmeOpts(app, r.BaseName()),
		BookmarksFromStore(r),
		StatsFromStore(r),
		Cipher(app),
	)

	if err := hook.Run(ctx, gm.Git()); err != nil {
		return fmt.Errorf("readme hook: %w", err)
	}

	return nil
}

func removeReadmes(ctx context.Context, app *application.App, dbs []string) error {
	root := app.Path.Git()

	rc, err := LoadRepoConfig(root)
	if err != nil {
		return err
	}

	cipher, err := app.Session().
		WithPassphrase("").
		WithValidation(false).
		NewCipher(ctx, root, "", rc.Encryption)
	if err != nil {
		return err
	}

	for _, dbPath := range dbs {
		name := files.StripExts(filepath.Base(dbPath))
		path := app.Path.GitReadme(name) + cipher.Suffix()

		if !files.Exists(path) {
			continue
		}

		if err := os.Remove(path); err != nil {
			return err
		}
	}

	return nil
}

// ReadmeCfg is everything the README needs from the application.
type ReadmeCfg struct {
	Name    string // database name, used in the title
	Path    string // where the README is written
	Glyphs  string
	Summary string
	Sort    string
}

func ReadmeOpts(app *application.App, repo string) *ReadmeCfg {
	cfg := app.Git.Readme
	return &ReadmeCfg{
		Name:    repo,
		Path:    app.Path.GitReadme(repo),
		Glyphs:  cfg.Glyphs,
		Summary: cfg.Summary,
		Sort:    app.Flags.Sort,
	}
}
