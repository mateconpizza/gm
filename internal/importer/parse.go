package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/repo"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

var ErrNotImplemented = errors.New("not implemented")

// deduplicate removes duplicate bookmarks.
func deduplicate(f *frame.Frame, r *repo.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	originalLen := bs.Len()
	bs.FilterInPlace(func(b *bookmark.Bookmark) bool {
		_, exists := r.Has(b.URL)
		return !exists
	})
	if originalLen != bs.Len() {
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-bs.Len())
		f.Warning(s + "\n").Flush()
	}

	if bs.Empty() {
		return slice.ErrSliceEmpty
	}

	return nil
}

// parseFoundInBrowser processes the bookmarks found from the import
// browser process.
func parseFoundInBrowser(
	t *terminal.Term,
	r *repo.SQLiteRepository,
	bs *slice.Slice[bookmark.Bookmark],
) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Rowln()
	if err := deduplicate(f, r, bs); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Midln("no new bookmark found, skipping import").Flush()
			return nil
		}
	}

	msg := fmt.Sprintf("scrape missing data from %d bookmarks found?", bs.Len())
	f.Rowln().Flush().Clear()
	if !config.App.Force {
		if err := t.ConfirmErr(f.Question(msg).String(), "y"); err != nil {
			if errors.Is(err, terminal.ErrActionAborted) {
				return nil
			}

			return fmt.Errorf("%w", err)
		}
	}

	if err := bookmark.ScrapeMissingDescription(bs); err != nil {
		return fmt.Errorf("scrapping missing description: %w", err)
	}
	return nil
}

// diffDeletedBookmarks checks for deleted bookmarks.
func diffDeletedBookmarks(root string, r *repo.SQLiteRepository, bookmarks []*bookmark.Bookmark) error {
	jsonBookmarks := slice.New[bookmark.Bookmark]()
	if err := bookmark.LoadJSONBookmarks(root, jsonBookmarks); err != nil {
		return fmt.Errorf("loading JSON bookmarks: %w", err)
	}
	diff := bookmark.FindChanged(bookmarks, jsonBookmarks.ItemsPtr())
	if len(diff) == 0 {
		return nil
	}

	for _, b := range diff {
		if _, ok := r.Has(b.URL); ok {
			continue
		}
		if err := bookmark.CleanupGitFiles(root, b, ".json"); err != nil {
			return fmt.Errorf("cleanup files: %w", err)
		}
	}
	return nil
}

func parseJSONRepo(root string) ([]*bookmark.Bookmark, error) {
	// FIX:
	_ = diffDeletedBookmarks(root, nil, nil)
	return nil, ErrNotImplemented
}

func parseGPGRepo(root string) ([]*bookmark.Bookmark, error) {
	var (
		count      = 0
		errTracker = bookmark.NewErrorTracker()
		wg         sync.WaitGroup
		mu         sync.Mutex
		bookmarks  = []*bookmark.Bookmark{}
	)
	sp := rotato.New(
		rotato.WithPrefix("Decrypting bookmarks"),
		rotato.WithMesgColor(rotato.ColorBrightBlue),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic, rotato.ColorStyleBold),
	)

	loader := func(path string) (*bookmark.Bookmark, error) {
		content, err := gpg.Decrypt(path)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		bj := &bookmark.BookmarkJSON{}
		if err := json.Unmarshal(content, bj); err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		if !bookmark.ValidateChecksumJSON(bj) {
			return nil, fmt.Errorf("%w: %s", bookmark.ErrInvalidChecksum, path)
		}

		count++
		sp.UpdateMesg(fmt.Sprintf("[%d] %s", count, filepath.Base(path)))

		b := bookmark.NewFromJSON(bj)
		bookmarks = append(bookmarks, b)

		return b, nil
	}

	sp.Start()

	err := filepath.WalkDir(root, parseGPGFile(&wg, &mu, loader))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	wg.Wait()
	err = errTracker.GetError()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	sp.UpdatePrefix(fmt.Sprintf("Decrypted %d bookmarks", count))
	sp.Done()

	return bookmarks, nil
}

func parseGitRepo(repoPath string) ([]*bookmark.Bookmark, error) {
	if !files.Exists(repoPath) {
		return nil, fmt.Errorf("%w: %q", git.ErrGitRepoNotFound, repoPath)
	}
	rootDir := filepath.Dir(repoPath)
	if !gpg.IsActive(rootDir) {
		fmt.Println("load as a JSON repository")
		return parseJSONRepo(repoPath)
	}

	return parseGPGRepo(repoPath)
}

// parseGPGFile is a WalkDirFunc that loads .gpg files concurrently.
func parseGPGFile(
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	loader func(path string) (*bookmark.Bookmark, error),
) fs.WalkDirFunc {
	var (
		bs                 = slice.New[bookmark.Bookmark]()
		count              = 0
		errTracker         = bookmark.NewErrorTracker()
		passphrasePrompted = false
		sem                = make(chan struct{}, runtime.NumCPU()*2)
	)

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".gpg" {
			return nil
		}
		// encrypt|decrypt the first item found, this will prompt the user
		// for the passphrase.
		if !passphrasePrompted {
			_, err = loader(path)
			if err != nil {
				return err
			}
			passphrasePrompted = true
			count--
			return nil
		}
		bookmark.LoadConcurrently(path, bs, wg, mu, sem, loader, errTracker)
		return nil
	}
}
