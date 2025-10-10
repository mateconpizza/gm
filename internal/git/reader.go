package git

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

type RepositoryReader struct {
	RepositoryReaderOpts
	Root string
	Path string
}

type RepositoryReaderOptFn func(*RepositoryReaderOpts)

type RepositoryReaderOpts struct {
	spinner *rotato.Rotato
}

func WithSpinner(sp *rotato.Rotato) RepositoryReaderOptFn {
	return func(o *RepositoryReaderOpts) {
		o.spinner = sp
	}
}

func (r *RepositoryReader) Read() ([]*bookmark.Bookmark, error) {
	loader := readJSONRepo
	if gpg.IsInitialized(r.Root) {
		loader = readGPGRepo
	}

	return loader(r.Path, r.spinner)
}

func NewReader(repoPath string, opts ...RepositoryReaderOptFn) *RepositoryReader {
	opt := RepositoryReaderOpts{}
	for _, fn := range opts {
		fn(&opt)
	}

	return &RepositoryReader{
		Root:                 filepath.Dir(repoPath),
		Path:                 repoPath,
		RepositoryReaderOpts: opt,
	}
}

var GPGStrategy = &bookio.RepositoryLoader{
	LoaderFn: gpgLoader,
	Prefix:   "Decrypting GPG bookmarks",
	FileFilter: bookio.And(
		bookio.IsFile,
		bookio.HasExtension(gpg.Extension),
		bookio.NotNamed(SummaryFileName),
	),
}

func gpgLoader(ctx context.Context, path string) (*bookmark.Bookmark, error) {
	app := config.New()
	fingerprintPath := gpg.GPGIDPath(app.Git.Path)
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

// ReadRepo is the unified function that uses the Strategy Pattern.
// It accepts a specific RepositoryLoader to delegate file loading and filtering.
func ReadRepo(root string, st *bookio.RepositoryLoader, sp *rotato.Rotato) ([]*bookmark.Bookmark, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := bookio.NewFileLoader(ctx)
	f.WithLoader(st.LoaderFn)
	if sp != nil {
		f.WithSpinner(sp)
	}
	f.Spinner.UpdatePrefix(st.Prefix)
	f.Spinner.Start()

	// Only for the GPGStrategy
	var passphrasePrompted bool

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: walking root: %s, on file: %s", err, root, path)
		}

		if !st.FileFilter(path, d) {
			return nil
		}

		// Prompt for GPG passphrase on the first valid file
		if st == GPGStrategy && !passphrasePrompted {
			f.Spinner.UpdateMesg("waiting for GPG passphrase")
			if _, err := f.Loader(f.Context, path); err != nil {
				return err
			}

			f.LoadAsync(path)

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

func readJSONRepo(root string, sp *rotato.Rotato) ([]*bookmark.Bookmark, error) {
	return ReadRepo(root, bookio.JSONStrategy, sp)
}

func readGPGRepo(root string, sp *rotato.Rotato) ([]*bookmark.Bookmark, error) {
	return ReadRepo(root, GPGStrategy, sp)
}

func readBookmarks(root, repoPath string) ([]*bookmark.Bookmark, error) {
	loader := readJSONRepo
	if gpg.IsInitialized(root) {
		loader = readGPGRepo
	}

	return loader(repoPath, nil)
}
