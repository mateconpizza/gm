package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

const JSONFileExt = ".json"

// storeBookmarkAsJSON creates files structure.
//
//	root -> dbName -> domain
//
// Returns true if the file was created or updated, false if no changes were made.
func storeBookmarkAsJSON(rootPath string, b *bookmark.Bookmark, force bool) (bool, error) {
	domain, err := b.Domain()
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// domainPath: root -> dbName -> domain
	domainPath := filepath.Join(rootPath, domain)
	if err := files.MkdirAll(domainPath); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// urlHash := domainPath -> urlHash.json
	urlHash := b.HashURL()
	filePathJSON := filepath.Join(domainPath, urlHash+JSONFileExt)

	updated, err := files.JSONWrite(filePathJSON, b.ToJSON(), force)
	if err != nil {
		return resolveFileConflictErr(rootPath, err, filePathJSON, b)
	}

	return updated, nil
}

// resolveFileConflictErr resolves a file conflict error.
// Returns true if the file was updated, false if no update was needed.
func resolveFileConflictErr(
	rootPath string,
	err error,
	filePathJSON string,
	b *bookmark.Bookmark,
) (bool, error) {
	if !errors.Is(err, files.ErrFileExists) {
		return false, err
	}

	bj := bookmark.BookmarkJSON{}
	if err := files.JSONRead(filePathJSON, &bj); err != nil {
		return false, fmt.Errorf("%w", err)
	}

	// no need to update
	if bj.Checksum == b.Checksum {
		return false, nil
	}

	return storeBookmarkAsJSON(rootPath, b, true)
}

// cleanGPGRepo removes the files from the git repo.
func cleanGPGRepo(root string, bs []*bookmark.Bookmark) error {
	slog.Debug("cleaning up git JSON files")

	for _, b := range bs {
		gpgPath, err := b.GPGPath()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fname := filepath.Join(root, gpgPath)
		if err := files.RemoveFilepath(fname); err != nil {
			if errors.Is(err, files.ErrFileNotFound) {
				return nil
			}

			return fmt.Errorf("cleaning GPG: %w", err)
		}
	}

	return nil
}

// cleanJSONRepo removes the files from the git repo.
func cleanJSONRepo(root string, bs []*bookmark.Bookmark) error {
	slog.Debug("cleaning up git JSON files")

	for _, b := range bs {
		jsonPath, err := b.JSONPath()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fname := filepath.Join(root, jsonPath)
		if err := files.RemoveFilepath(fname); err != nil {
			return fmt.Errorf("cleaning JSON: %w", err)
		}
	}

	return nil
}

// writeRepoStats updates the repo stats.
func writeRepoStats(gr *Repository) error {
	var (
		summary     *SyncGitSummary
		summaryPath = filepath.Join(gr.Loc.Path, SummaryFileName)
	)

	if !files.Exists(summaryPath) {
		// Create new summary with only RepoStats
		summary = NewSummary()
		if err := repoStats(gr.Loc.DBPath, summary); err != nil {
			return fmt.Errorf("creating repo stats: %w", err)
		}
	} else {
		// Load existing summary
		summary = NewSummary()
		if err := files.JSONRead(summaryPath, summary); err != nil {
			return fmt.Errorf("reading summary: %w", err)
		}
		// Update only RepoStats
		if err := repoStats(gr.Loc.DBPath, summary); err != nil {
			return fmt.Errorf("updating repo stats: %w", err)
		}
	}

	// Save updated or new summary
	if _, err := files.JSONWrite(summaryPath, summary, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	return nil
}

// repoStats returns a new RepoStats.
func repoStats(dbPath string, summary *SyncGitSummary) error {
	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	summary.RepoStats = &RepoStats{
		Name:      r.Cfg.Name,
		Bookmarks: db.CountMainRecords(r),
		Tags:      db.CountTagsRecords(r),
		Favorites: db.CountFavorites(r),
	}

	summary.GenChecksum()

	return nil
}

// syncSummary returns a new SyncGitSummary.
func syncSummary(gr *Repository) (*SyncGitSummary, error) {
	r, err := db.New(gr.Loc.DBPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}

	branch, err := gr.Git.Branch()
	if err != nil {
		return nil, fmt.Errorf("getting branch: %w", err)
	}

	remote, err := gr.Git.Remote()
	if err != nil {
		remote = ""
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("getting hostname: %w", err)
	}

	summary := &SyncGitSummary{
		GitBranch:          branch,
		GitRemote:          remote,
		LastSync:           time.Now().Format(time.RFC3339),
		ConflictResolution: "timestamp",
		HashAlgorithm:      "SHA-256",
		ClientInfo: &ClientInfo{
			Hostname:   hostname,
			Platform:   runtime.GOOS,
			Architect:  runtime.GOARCH,
			AppVersion: config.App.Info.Version,
		},
		RepoStats: &RepoStats{
			Name:      r.Cfg.Name,
			Bookmarks: db.CountMainRecords(r),
			Tags:      db.CountTagsRecords(r),
			Favorites: db.CountFavorites(r),
		},
	}

	summary.GenChecksum()

	return summary, nil
}

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(gr *Repository, actionMsg string) error {
	err := writeRepoStats(gr)
	if err != nil {
		return err
	}

	gm := gr.Git

	// check if any changes
	changed, _ := gm.HasChanges()
	if !changed {
		return nil
	}

	if err := gm.AddAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	status, err := gm.Status()
	if err != nil {
		status = ""
	}
	if status != "" {
		status = "(" + status + ")"
	}

	msg := fmt.Sprintf("[%s] %s %s", gr.Loc.DBName, actionMsg, status)
	if err := gm.Commit(msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// repoSummaryString returns a string representation of the repo summary.
func repoSummaryString(sum *SyncGitSummary) string {
	st := sum.RepoStats
	var parts []string
	if st.Bookmarks > 0 {
		parts = append(parts, fmt.Sprintf("%d bookmarks", st.Bookmarks))
	}

	if st.Tags > 0 {
		parts = append(parts, fmt.Sprintf("%d tags", st.Tags))
	}

	if st.Favorites > 0 {
		parts = append(parts, fmt.Sprintf("%d favorites", st.Favorites))
	}

	if len(parts) == 0 {
		parts = append(parts, "no bookmarks")
	}

	return strings.Join(parts, ", ")
}

// records gets all records from the database.
func records(dbPath string) ([]*bookmark.Bookmark, error) {
	r, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}
	bs, err := r.AllPtr()
	if err != nil {
		return nil, fmt.Errorf("getting bookmarks: %w", err)
	}
	return bs, nil
}

// parseGitRepository loads a git repo into a database.
func parseGitRepository(c *ui.Console, root, repoName string) (string, error) {
	c.F.Rowln().Info(fmt.Sprintf(color.Text("Repository %q\n").Bold().String(), repoName))
	repoPath := filepath.Join(root, repoName)

	// read summary.json
	sum := NewSummary()
	if err := files.JSONRead(filepath.Join(repoPath, SummaryFileName), sum); err != nil {
		return "", fmt.Errorf("reading summary: %w", err)
	}

	c.F.Midln(txt.PaddedLine("records:", sum.RepoStats.Bookmarks)).
		Midln(txt.PaddedLine("tags:", sum.RepoStats.Tags)).
		Midln(txt.PaddedLine("favorites:", sum.RepoStats.Favorites)).Flush()

	if err := c.ConfirmErr("Import records from this repo?", "y"); err != nil {
		return "", fmt.Errorf("%w", err)
	}

	var (
		dbName = sum.RepoStats.Name
		dbPath = filepath.Join(config.App.Path.Data, dbName)
		opt    string
		err    error
	)

	if files.Exists(dbPath) {
		c.Warning(fmt.Sprintf("Database %q already exists\n", dbName)).Flush()

		opt, err = c.Choose(
			"What do you want to do?",
			[]string{"merge", "drop", "create", "select", "ignore"},
			"m",
		)
		if err != nil {
			return "", fmt.Errorf("%w", err)
		}
	} else {
		opt = "new"
	}

	resultPath, err := parseGitRepositoryOpt(c, opt, dbPath, repoPath)
	if err != nil {
		return "", err
	}

	return resultPath, nil
}

// parseGitRepositoryOpt handles the options for parseGitRepository.
func parseGitRepositoryOpt(c *ui.Console, o, dbPath, repoPath string) (string, error) {
	switch strings.ToLower(o) {
	case "new":
		if err := intoDBFromGit(c, dbPath, repoPath); err != nil {
			return "", err
		}

	case "c", "create":
		var dbName string
		for dbName == "" {
			dbName = files.EnsureSuffix(c.Prompt("Enter new name: "), ".db")
		}

		dbPath = filepath.Join(filepath.Dir(dbPath), dbName)
		if err := intoDBFromGit(c, dbPath, repoPath); err != nil {
			return "", err
		}

	case "d", "drop":
		c.Warning("Dropping database\n").Flush()
		if err := db.DropFromPath(dbPath); err != nil {
			return "", fmt.Errorf("%w", err)
		}
		if err := mergeAndInsert(c, dbPath, repoPath); err != nil {
			return "", err
		}

	case "m", "merge":
		c.Info("Merging database\n").Flush()
		if err := mergeAndInsert(c, dbPath, repoPath); err != nil {
			return "", err
		}

	case "s", "select":
		if err := selectAndInsert(c, dbPath, repoPath); err != nil {
			if errors.Is(err, menu.ErrFzfActionAborted) {
				return "", nil
			}

			return "", err
		}

	case "i", "ignore":
		repoName := files.StripSuffixes(filepath.Base(dbPath))
		c.ReplaceLine(
			c.Warning(fmt.Sprintf("%s repo %q", color.Yellow("skipping"), repoName)).StringReset(),
		)

		return "", nil
	}

	return dbPath, nil
}

// readJSONRepo extracts records from a JSON repository.
func readJSONRepo(c *ui.Console, root string) ([]*bookmark.Bookmark, error) {
	var (
		count      = 0
		errTracker = NewErrorTracker()
		wg         sync.WaitGroup
		mu         sync.Mutex
		bookmarks  = []*bookmark.Bookmark{}
	)

	sp := rotato.New(
		rotato.WithPrefix(c.F.Mid("Loading JSON bookmarks").String()),
		rotato.WithMesgColor(rotato.ColorBrightBlue),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
	)

	loader := func(path string) (*bookmark.Bookmark, error) {
		bj := &bookmark.BookmarkJSON{}
		if err := files.JSONRead(path, bj); err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		if !bookmark.ValidateChecksumJSON(bj) {
			return nil, fmt.Errorf("%w: %s", bookmark.ErrInvalidChecksum, path)
		}

		count++
		sp.UpdateMesg(fmt.Sprintf("[%d] %s", count, filepath.Base(path)))

		b := bookmark.NewFromJSON(bj)

		mu.Lock()
		bookmarks = append(bookmarks, b)
		mu.Unlock()

		return b, nil
	}

	sp.Start()

	ctx := context.Background()
	err := filepath.WalkDir(root, parseJSONFile(ctx, &wg, &mu, loader))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	wg.Wait()

	err = errTracker.GetError()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	sp.UpdatePrefix(c.Success(fmt.Sprintf("Loaded %d bookmarks", count)).String())
	sp.Done("done!")

	return bookmarks, nil
}

// readGPGRepo extracts records from a GPG repository.
func readGPGRepo(c *ui.Console, root string) ([]*bookmark.Bookmark, error) {
	var (
		count      = 0
		errTracker = NewErrorTracker()
		wg         sync.WaitGroup
		mu         sync.Mutex
		bookmarks  = []*bookmark.Bookmark{}
	)

	sp := rotato.New(
		rotato.WithPrefix(c.F.Mid("Decrypting bookmarks").StringReset()),
		rotato.WithMesgColor(rotato.ColorBrightBlue),
		rotato.WithDoneColorMesg(rotato.ColorBrightGreen, rotato.ColorStyleItalic),
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

	ctx := context.Background()
	err := filepath.WalkDir(root, parseGPGFile(ctx, &wg, &mu, loader))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	wg.Wait()

	err = errTracker.GetError()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	sp.UpdatePrefix(c.Success(fmt.Sprintf("Decrypted %d bookmarks", count)).StringReset())
	sp.Done("done!")

	return bookmarks, nil
}

// parseGPGFile is a WalkDirFunc that loads .gpg files concurrently using a semaphore.
func parseGPGFile(
	ctx context.Context,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	loader loaderFileFn,
) fs.WalkDirFunc {
	var (
		bs                 []*bookmark.Bookmark
		passphrasePrompted bool
		errTracker         = NewErrorTracker()
		sem                = semaphore.NewWeighted(int64(runtime.NumCPU() * 2))
	)

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != gpg.Extension {
			return nil
		}

		// Prompt for GPG passphrase on the first valid file
		if !passphrasePrompted {
			if _, err := loader(path); err != nil {
				return err
			}
			passphrasePrompted = true
			return nil
		}

		loadConcurrently(ctx, path, &bs, wg, mu, sem, loader, errTracker)
		return nil
	}
}

func parseJSONFile(
	ctx context.Context,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	loader loaderFileFn,
) fs.WalkDirFunc {
	bs := []*bookmark.Bookmark{}
	errTracker := NewErrorTracker()
	sem := semaphore.NewWeighted(int64(runtime.NumCPU() * 2))

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != JSONFileExt {
			return nil
		}

		loadConcurrently(ctx, path, &bs, wg, mu, sem, loader, errTracker)
		return nil
	}
}
