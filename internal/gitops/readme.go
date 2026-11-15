package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

var ErrGitReadmeDisable = errors.New("git: readme disable")

func EnableReadme(ctx context.Context, app *application.App, c console) error {
	return setReadme(ctx, app, c, true)
}

func DisableReadme(ctx context.Context, app *application.App, c console) error {
	return setReadme(ctx, app, c, false)
}

func ConfigureReadme(ctx context.Context, app *application.App, c console) error {
	return setupReadme(ctx, app, c)
}

func ViewReadme(ctx context.Context, app *application.App) error { return readReadme(ctx, app) }

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

// setReadme updates the README configuration and synchronizes Git
// repositories.
func setReadme(ctx context.Context, app *application.App, c console, enabled bool) error {
	log := app.Logger().With("fn", "set-readme", "enabled", enabled)

	app.Git.Readme.Enabled = enabled

	gm := app.Git.Manager()
	g := gm.Git()

	update := removeReadmes
	msg := "del readme"

	if enabled {
		update = generateReadmes
		msg = "add readme"
	}

	log.Debug("updating readmes")
	if err := update(ctx, app, app.Databases()); err != nil {
		return err
	}

	if err := HookHasChanges().Run(ctx, g); err != nil {
		return err
	}

	log.Debug("writing config")
	if err := app.WriteConfig(true); err != nil {
		return err
	}

	log.Debug("committing", "msg", msg)
	if err := gm.Commit(ctx, CoreCommitMsg(msg)); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("settings saved\n"))
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

	r, err := db.Open(ctx, path)
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
		app.Logger(),
	)

	if err := hook.Run(ctx, gm.Git()); err != nil {
		return fmt.Errorf("readme hook: %w", err)
	}

	return nil
}

func removeReadmes(ctx context.Context, app *application.App, dbs []string) error {
	root := app.Path.Git()

	rc, err := resolveConfig(root)
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

func readReadme(ctx context.Context, app *application.App) error {
	log := app.Logger().With("fn", "read-readme", "db", app.DBBaseName())

	c, err := Cipher(app)(ctx)
	if err != nil {
		return err
	}

	p := app.Path.GitReadme(app.DBBaseName()) + c.Suffix()
	log.Debug("reading readme", "file", filepath.Base(p), "suffix", c.Suffix())

	if err := files.ExistsErr(p); err != nil {
		return fmt.Errorf("%w: %q", err, p)
	}

	data, err := c.Read(ctx, p)
	if err != nil {
		return err
	}
	log.Debug("readme read", "bytes", len(data))

	fmt.Fprint(app.Git.Writer(), string(data))

	return nil
}

func setupReadme(ctx context.Context, app *application.App, c console) error {
	log := app.Logger().With("fn", "setup-readme", "db", app.DBBaseName())

	r := app.Git.Readme
	log.Debug("setting up readme", "enabled", r.Enabled, "summary", r.Summary, "glyphs", r.Glyphs)

	sumOpt, err := c.Choose(ctx, "Which summary style?", bookio.SupportedSummary, "badges")
	if err != nil {
		return err
	}
	r.Summary = sumOpt

	glyphs := slices.DeleteFunc(slices.Clone(bookio.SupportedGlyphs), func(s string) bool {
		return s == "none"
	})
	glyphOpt, err := c.Choose(ctx, "Which glyph style?", glyphs, "emoji")
	if err != nil {
		return err
	}
	r.Glyphs = glyphOpt

	log.Debug("saving readme settings", "summary", r.Summary, "glyphs", r.Glyphs)

	if err := app.WriteConfig(true); err != nil {
		return err
	}

	return setReadme(ctx, app, c, r.Enabled)
}
