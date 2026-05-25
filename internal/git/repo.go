package git

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type RepoOptFn func(*RepoOpts)

type RepoOpts struct {
	spinner *rotato.Rotato
	ctx     context.Context
}

type Repo struct {
	name      string
	fullpath  string
	bookmarks []*bookmark.Bookmark
	stats     *db.RepoStats

	RepoOpts
}

func (r *Repo) Root() string {
	return filepath.Dir(r.fullpath)
}

func (r *Repo) Stats() *db.RepoStats {
	return r.stats
}

// LoadSummary loads summary file found in repository.
func (r *Repo) LoadSummary() error {
	sum, err := loadSummary(r.fullpath)
	if err != nil {
		return err
	}

	r.stats = sum.RepoStats

	return nil
}

// Read reads bookmarks in git repository.
func (r *Repo) Read(ctx context.Context) error {
	loader := readJSONRepo
	if gpg.IsInitialized(filepath.Dir(r.fullpath)) {
		loader = readGPGRepo
	}

	bs, err := loader(ctx, r.fullpath, r.spinner)
	if err != nil {
		return err
	}

	slices.SortFunc(bs, func(a, b *bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})

	r.bookmarks = bs

	return nil
}

func (r *Repo) Name() string {
	return r.name
}

func (r *Repo) Bookmarks() []*bookmark.Bookmark {
	return r.bookmarks
}

func (r *Repo) String() string {
	return txt.PaddedLine(r.name, fmt.Sprintf("(bookmarks: %d)", r.stats.Bookmarks))
}

func NewRepo(name, dstDir string, opts ...RepoOptFn) *Repo {
	opt := RepoOpts{}
	for _, fn := range opts {
		fn(&opt)
	}

	if opt.ctx == nil {
		opt.ctx = context.Background()
	}

	return &Repo{
		name:     name,
		fullpath: dstDir,
		RepoOpts: opt,
	}
}

var GPGStrategy = &bookio.RepositoryLoader{
	Func:   gpgLoader,
	Prefix: "Decrypting GPG bookmarks",
	FileFilter: bookio.And(
		bookio.IsFile,
		bookio.HasExtension(gpg.Extension),
		bookio.NotNamed(SummaryFileName),
	),
}

func gpgLoader(ctx context.Context, path string) (*bookmark.Bookmark, error) {
	// FIX: getting `fingerprintPath`

	// path: [root/repoName/domainName/fileName.gpg]
	// root: [root]
	fingerprintPath := gpg.GPGIDPath(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	content, err := gpg.Decrypt(ctx, fingerprintPath, path)
	if err != nil {
		return nil, fmt.Errorf("decrypting %w", err)
	}

	bj := &bookmark.BookmarkJSON{}
	if err := json.Unmarshal(content, bj); err != nil {
		fmt.Println(string(content))
		fmt.Println(path)
		return nil, fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	return bookmark.NewFromJSON(bj), nil
}

func readJSONRepo(ctx context.Context, root string, sp *rotato.Rotato) ([]*bookmark.Bookmark, error) {
	return ReadRepo(ctx, RepoConfig{
		Root:    root,
		Loader:  bookio.JSONStrategy,
		Spinner: sp,
	})
}

func readGPGRepo(ctx context.Context, root string, sp *rotato.Rotato) ([]*bookmark.Bookmark, error) {
	return ReadRepo(ctx, RepoConfig{
		Root:    root,
		Loader:  GPGStrategy,
		Spinner: sp,
	})
}

// RepoConfig groups the configuration needed to read a repository.
type RepoConfig struct {
	Root    string
	Loader  *bookio.RepositoryLoader
	Spinner *rotato.Rotato
}

// ReadRepo is the unified function that uses the Strategy Pattern.
func ReadRepo(ctx context.Context, cfg RepoConfig) ([]*bookmark.Bookmark, error) {
	f := bookio.NewFileLoader(ctx)
	f.WithLoader(cfg.Loader.Func)
	if cfg.Spinner != nil {
		f.WithSpinner(cfg.Spinner)
	}

	f.Spinner.UpdatePrefix(cfg.Loader.Prefix)
	f.Spinner.Start()

	// Only for the GPGStrategy
	var passphrasePrompted bool

	err := filepath.WalkDir(cfg.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: walking root: %s, on file: %s", err, cfg.Root, path)
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if !cfg.Loader.FileFilter(path, d) {
			return nil
		}

		// Handle prompt for GPG passphrase on the first valid file
		if cfg.Loader == GPGStrategy && !passphrasePrompted {
			if err := promptGPGPassphrase(ctx, f, path); err != nil {
				return err
			}
			passphrasePrompted = true
			return nil
		}

		f.LoadAsync(path)
		return nil
	})
	if err != nil {
		f.Spinner.Fail(err.Error())
		return nil, err
	}

	return f.Results()
}

// promptGPGPassphrase handles unlocking and initializing the first GPG file.
func promptGPGPassphrase(ctx context.Context, f *bookio.FileLoader, path string) error {
	unlocked, err := gpg.Unlocked(f.Context, path)
	if err != nil {
		return err
	}

	if unlocked {
		if _, err := f.Loader(ctx, path); err != nil {
			return err
		}
		f.LoadAsync(path)
		return nil
	}

	ctxPrompt, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	f.Spinner.UpdateMesg("waiting for GPG passphrase")

	if _, err := f.Loader(ctxPrompt, path); err != nil {
		return err
	}

	f.LoadAsync(path)
	return nil
}
