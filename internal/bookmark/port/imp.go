package port

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

// GitImport imports bookmarks from a git repository.
func GitImport(c *ui.Console, gm *git.Manager, urlRepo string) ([]string, error) {
	if err := gm.Clone(urlRepo); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	repos, err := files.ListRootFolders(gm.RepoPath, ".git")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	n := len(repos)
	if n == 0 {
		return nil, git.ErrGitRepoNotFound
	}

	var imported []string

	c.F.Midln(fmt.Sprintf("Found %d repositorie/s", n)).Flush()

	for _, repoName := range repos {
		dbPath, err := parseGitRepository(c, gm.RepoPath, repoName)
		if err != nil {
			if errors.Is(err, terminal.ErrActionAborted) {
				c.ReplaceLine(
					c.Warning(fmt.Sprintf("%s repo %q", color.Yellow("skipping"), repoName)).StringReset(),
				)

				n--

				continue
			}

			return nil, fmt.Errorf("%w", err)
		}

		if dbPath != "" {
			imported = append(imported, dbPath)
		}
	}

	if len(imported) == 0 {
		return nil, terminal.ErrActionAborted
	}

	if err := files.RemoveAll(gm.RepoPath); err != nil {
		return nil, fmt.Errorf("removing temp repo: %w", err)
	}

	fmt.Print(c.SuccessMesg("imported bookmarks from git\n"))

	return imported, nil
}

// Browser imports bookmarks from a supported browser.
func Browser(c *ui.Console, r *db.SQLiteRepository) error {
	br, ok := getBrowser(selectBrowser(c))
	if !ok {
		return fmt.Errorf("%w", browser.ErrBrowserUnsupported)
	}

	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}
	// find bookmarks
	bs, err := br.Import(c, config.App.Flags.Force)
	if err != nil {
		return fmt.Errorf("browser %q: %w", br.Name(), err)
	}

	// clean and process found bookmarks
	bs, err = parseFoundInBrowser(c, r, bs)
	if err != nil {
		return err
	}

	if len(bs) == 0 {
		return nil
	}

	return IntoRepo(c, r, bs)
}

// Database imports bookmarks from a database.
func Database(c *ui.Console, srcDB, destDB *db.SQLiteRepository) error {
	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import", false),
		menu.WithPreview(config.App.Cmd+" -n "+srcDB.Name()+" records {1}"),
		menu.WithInterruptFn(func(err error) { // build interrupt cleanup
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}),
	)

	items, err := srcDB.All()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	m.SetItems(items)
	m.SetPreprocessor(bookmark.Oneline)

	records, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bookmarks := make([]*bookmark.Bookmark, 0, len(records))
	for i := range records {
		bookmarks = append(bookmarks, &records[i])
	}

	bookmarks = deduplicatePtr(c, destDB, bookmarks)
	n := len(bookmarks)
	if n == 0 {
		c.F.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	if err := destDB.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg(fmt.Sprintf("imported %d record/s from %s\n", n, srcDB.Name())))

	return nil
}

// IntoRepo import records into the database.
func IntoRepo(c *ui.Console, r *db.SQLiteRepository, records []*bookmark.Bookmark) error {
	n := len(records)
	if !config.App.Flags.Force && n > 1 {
		if err := c.ConfirmErr(fmt.Sprintf("import %d records?", n), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp := rotato.New(
		rotato.WithMesg("importing record/s..."),
		rotato.WithMesgColor(rotato.ColorYellow),
	)
	sp.Start()

	if err := r.InsertMany(context.Background(), records); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Done()

	fmt.Print(c.SuccessMesg(fmt.Sprintf("imported %d record/s\n", n)))

	return nil
}

// FromBackup imports bookmarks from a backup.
func FromBackup(c *ui.Console, destDB, srcDB *db.SQLiteRepository) error {
	s := color.BrightYellow("Import bookmarks from backup: ").String()
	c.F.Headerln(s + color.Gray(srcDB.Name()).Italic().String()).Flush()
	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithMultiSelection(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithPreview(fmt.Sprintf("%s -n ./backup/%s {1}", config.App.Cmd, srcDB.Name())),
		menu.WithInterruptFn(c.T.InterruptFn),
		menu.WithHeader("select record/s to import from '"+srcDB.Name()+"'", false),
	)

	defer c.T.CancelInterruptHandler()

	bookmarks, err := srcDB.All()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	m.SetItems(bookmarks)
	m.SetPreprocessor(bookmark.Oneline)

	items, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	result := make([]*bookmark.Bookmark, 0, len(items))
	for i := range items {
		result = append(result, &items[i])
	}

	dRecords := deduplicatePtr(c, destDB, result)
	if len(dRecords) == 0 {
		c.F.Midln("no new bookmark found, skipping import").Flush()
		return nil
	}

	return IntoRepo(c, destDB, dRecords)
}

// mergeRecords merges non-duplicates records into database.
func mergeRecords(c *ui.Console, dbPath, repoPath string) error {
	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := extractFromGitRepo(c, repoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	bookmarks = deduplicatePtr(c, r, bookmarks)
	if err := r.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	n := len(bookmarks)
	if n > 0 {
		c.F.Success(fmt.Sprintf("Imported %d records into %q\n", n, filepath.Base(dbPath))).Flush()
	}

	return nil
}

// intoDBFromGit import into database.
func intoDBFromGit(c *ui.Console, dbPath, repoPath string) error {
	bookmarks, err := extractFromGitRepo(c, repoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	r, err := db.Init(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	if err := r.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	if err := r.InsertMany(context.Background(), bookmarks); err != nil {
		return fmt.Errorf("%w", err)
	}

	c.F.Success(fmt.Sprintf("Imported %d records into %q\n", len(bookmarks), filepath.Base(dbPath))).Flush()

	return nil
}

func selectRecords(c *ui.Console, dbPath, repoPath string) error {
	bookmarks, err := extractFromGitRepo(c, repoPath)
	if err != nil {
		return err
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import", false),
	)

	records := make([]bookmark.Bookmark, 0, len(bookmarks))
	for _, b := range bookmarks {
		records = append(records, *b)
	}

	slices.SortFunc(records, func(a, b bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})

	m.SetItems(records)
	m.SetPreprocessor(bookmark.Oneline)

	selected, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bs := slice.New(selected...)

	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	debookmarks, err := Deduplicate(c, r, bs)
	if err != nil {
		return err
	}

	n := len(debookmarks)
	if n > 0 {
		c.F.Success(fmt.Sprintf("Imported %d records into %q\n", n, filepath.Base(dbPath))).Flush()
	}

	return nil
}

// loadConcurrently processes a single JSON file in a goroutine.
func loadConcurrently(
	path string,
	bs *slice.Slice[bookmark.Bookmark],
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	sem chan struct{},
	loader func(path string) (*bookmark.Bookmark, error),
	errTracker *ErrTracker,
) {
	// FIX: replace slice with []*Bookmark
	_ = mu
	sem <- struct{}{} // acquire semaphore

	wg.Add(1)

	go func(filePath string) {
		defer func() {
			<-sem     // release semaphore
			wg.Done() // mark goroutine as done
		}()

		b, err := loader(filePath)
		if err != nil {
			errTracker.SetError(err)
			return
		}

		bs.Push(b)
	}(path)
}
