package port

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

// GitStore saves the bookmark to the git repo as a file.
func GitStore(b *bookmark.Bookmark) error {
	repoPath := config.App.Path.Git
	if !git.IsInitialized(repoPath) {
		return nil
	}
	fileExt := FileExtJSON
	if gpg.IsInitialized(repoPath) {
		fileExt = gpg.Extension
	}

	root := filepath.Join(repoPath, files.StripSuffixes(config.App.DBName))

	switch fileExt {
	case FileExtJSON:
		return gitStoreAsJSON(root, b, config.App.Force)
	case gpg.Extension:
		return exportAsGPG(root, []*bookmark.Bookmark{b})
	}

	return nil
}

// GitUpdate updates the git repo.
func GitUpdate(dbPath string, newB, oldB *bookmark.Bookmark) error {
	repoPath := config.App.Path.Git
	if !git.IsInitialized(repoPath) {
		return nil
	}

	fileExt := FileExtJSON
	if gpg.IsInitialized(repoPath) {
		fileExt = gpg.Extension
	}

	dbName := files.StripSuffixes(filepath.Base(dbPath))
	root := filepath.Join(repoPath, dbName)

	switch fileExt {
	case FileExtJSON:
		return gitUpdateJSON(root, oldB, newB)
	case gpg.Extension:
		return GitCleanGPG(root, []*bookmark.Bookmark{newB})
	}

	return nil
}

// GitExport exports the bookmarks to the git repo.
func GitExport(dbPath string) error {
	if !git.IsInitialized(config.App.Path.Git) {
		slog.Debug("git export: git not initialized")
		return nil
	}

	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := r.AllPtr()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(bookmarks) == 0 {
		return git.ErrGitNothingToCommit
	}

	repoPath := config.App.Path.Git
	dbName := filepath.Base(dbPath)
	root := filepath.Join(repoPath, files.StripSuffixes(dbName))

	if err := GitWrite(repoPath, root, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// GitWrite exports the bookmarks to the git repo.
func GitWrite(repoPath, root string, bookmarks []*bookmark.Bookmark) error {
	if gpg.IsInitialized(repoPath) {
		if err := exportAsGPG(root, bookmarks); err != nil {
			return fmt.Errorf("store as GPG: %w", err)
		}

		return nil
	}

	return exportAsJSON(root, bookmarks)
}

// gitStoreAsJSON creates files structure.
//
//	root -> dbName -> domain
func gitStoreAsJSON(rootPath string, b *bookmark.Bookmark, force bool) error {
	domain, err := b.Domain()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	// domainPath: root -> dbName -> domain
	domainPath := filepath.Join(rootPath, domain)
	if err := files.MkdirAll(domainPath); err != nil {
		return fmt.Errorf("%w", err)
	}
	// urlHash := domainPath -> urlHash.json
	urlHash := b.HashURL()
	filePathJSON := filepath.Join(domainPath, urlHash+FileExtJSON)
	if err := files.JSONWrite(filePathJSON, b.ToJSON(), force); err != nil {
		return resolveFileConflictErr(rootPath, err, filePathJSON, b)
	}

	return nil
}

// exportAsJSON creates the repository structure.
func exportAsJSON(root string, bs []*bookmark.Bookmark) error {
	for _, b := range bs {
		if err := gitStoreAsJSON(root, b, config.App.Force); err != nil {
			return err
		}
	}

	return nil
}

// exportAsGPG export and encrypts the bookmarks and stores them in the git
// repo.
func exportAsGPG(root string, bookmarks []*bookmark.Bookmark) error {
	if err := files.MkdirAll(root); err != nil {
		return fmt.Errorf("%w", err)
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	sp := rotato.New(
		rotato.WithPrefix(f.Mid("Encrypting").String()),
		rotato.WithMesg("bookmarks..."),
		rotato.WithMesgColor(rotato.ColorYellow),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
		rotato.WithFailColorMesg(rotato.ColorBrightRed),
	)

	n := len(bookmarks)
	count := 0
	for i := range n {
		hashPath, err := bookmarks[i].HashPath()
		if err != nil {
			return fmt.Errorf("hashing path: %w", err)
		}
		if err := gpg.Create(root, hashPath, bookmarks[i].ToJSON()); err != nil {
			if errors.Is(err, files.ErrFileExists) {
				continue
			}
			return fmt.Errorf("creating GPG file: %w", err)
		}
		sp.Start()
		count++
		sp.UpdatePrefix(f.Reset().Mid(fmt.Sprintf("Encrypting [%d/%d]", count, n)).String())
	}

	if count > 0 {
		sp.Done("done")
	} else {
		sp.Done()
	}

	return nil
}

// exportFromGit extracts records from a git repository.
func exportFromGit(f *frame.Frame, repoPath string) ([]*bookmark.Bookmark, error) {
	if !files.Exists(repoPath) {
		return nil, fmt.Errorf("%w: %q", git.ErrGitRepoNotFound, repoPath)
	}
	rootDir := filepath.Dir(repoPath)
	if !gpg.IsInitialized(rootDir) {
		return parseJSONRepo(f, repoPath)
	}

	return parseGPGRepo(f, repoPath)
}

// ToJSON converts an interface to JSON.
func ToJSON(data any) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return jsonData, nil
}
