// Package port provides functionalities for importing and exporting data,
// supporting various sources and formats including browsers, databases, Git
// repositories, JSON, and GPG encrypted files.
package port

import (
	"encoding/json"
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
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

// GitStore saves the bookmark to the git repo as a file.
func GitStore(b *bookmark.Bookmark) error {
	repoPath := config.App.Path.Git
	if !git.IsInitialized(repoPath) {
		return nil
	}

	fileExt := JSONFileExt
	if gpg.IsInitialized(repoPath) {
		fileExt = gpg.Extension
	}

	root := filepath.Join(repoPath, files.StripSuffixes(config.App.DBName))

	switch fileExt {
	case JSONFileExt:
		return gitStoreAsJSON(root, b, config.App.Flags.Force)
	case gpg.Extension:
		return exportAsGPG(root, []*bookmark.Bookmark{b})
	}

	return nil
}

// GitUpdate updates the git repo.
func GitUpdate(gm *git.Manager, newB, oldB *bookmark.Bookmark) error {
	if !gm.IsInitialized() {
		return nil
	}

	fileExt := JSONFileExt
	if gpg.IsInitialized(gm.RepoPath) {
		fileExt = gpg.Extension
	}

	gr := gm.Tracker.Current()

	switch fileExt {
	case JSONFileExt:
		return gitUpdateJSON(gr.Path, oldB, newB)
	case gpg.Extension:
		return GitCleanGPG(gr.Path, []*bookmark.Bookmark{newB})
	}

	return nil
}

// GitExport exports the bookmarks to the git repo.
func GitExport(gm *git.Manager) error {
	if !gm.IsInitialized() {
		slog.Debug("git export: git not initialized")
		return nil
	}

	r, err := db.New(gm.Tracker.Current().DBPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := r.AllPtr()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := GitWrite(gm, bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// GitWrite exports the bookmarks to the git repo.
func GitWrite(gm *git.Manager, bookmarks []*bookmark.Bookmark) error {
	root := gm.Tracker.Current().Path
	if gpg.IsInitialized(gm.RepoPath) {
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

	filePathJSON := filepath.Join(domainPath, urlHash+JSONFileExt)
	if err := files.JSONWrite(filePathJSON, b.ToJSON(), force); err != nil {
		return resolveFileConflictErr(rootPath, err, filePathJSON, b)
	}

	return nil
}

// exportAsJSON creates the repository structure.
func exportAsJSON(root string, bs []*bookmark.Bookmark) error {
	for _, b := range bs {
		if err := gitStoreAsJSON(root, b, config.App.Flags.Force); err != nil {
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

		filePath := filepath.Join(root, hashPath+gpg.Extension)
		if files.Exists(filePath) {
			continue
		}

		dir := filepath.Dir(filePath)
		if err := files.MkdirAll(dir); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}

		data, err := json.MarshalIndent(bookmarks[i].ToJSON(), "", "  ")
		if err != nil {
			return fmt.Errorf("json marshal: %w", err)
		}

		if err := gpg.Encrypt(filePath, data); err != nil {
			return fmt.Errorf("%w", err)
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

// extractFromGitRepo extracts records from a git repository.
func extractFromGitRepo(c *ui.Console, repoPath string) ([]*bookmark.Bookmark, error) {
	if !files.Exists(repoPath) {
		return nil, fmt.Errorf("%w: %q", git.ErrGitRepoNotFound, repoPath)
	}

	rootDir := filepath.Dir(repoPath)
	if !gpg.IsInitialized(rootDir) {
		return parseJSONRepo(c, repoPath)
	}

	return parseGPGRepo(c, repoPath)
}

// ToJSON converts an interface to JSON.
func ToJSON(data any) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return jsonData, nil
}
