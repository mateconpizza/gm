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
	"github.com/mateconpizza/gm/internal/browser"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

// GitImport imports bookmarks from a git repository.
func GitImport(t *terminal.Term, f *frame.Frame, tmpPath, repoPath string) error {
	if err := git.Clone(tmpPath, repoPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	repos, err := files.ListRootFolders(tmpPath, ".git")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	n := len(repos)
	if n == 0 {
		return git.ErrGitRepoNotFound
	}
	f.Midln(fmt.Sprintf("Found %d repositorie/s", n)).Flush()

	for _, repoName := range repos {
		if err := parseGitRepository(tmpPath, repoName, t, f.Clear()); err != nil {
			if errors.Is(err, terminal.ErrActionAborted) {
				t.ClearLine(1)
				f.Clear().Warning(fmt.Sprintf("skipping repo %q\n", repoName)).Flush()
				n--
				continue
			}
			return fmt.Errorf("%w", err)
		}
	}

	if n == 0 {
		return terminal.ErrActionAborted
	}

	if err := files.RemoveAll(tmpPath); err != nil {
		return fmt.Errorf("removing temp repo: %w", err)
	}

	f.Clear().Rowln().
		Success(color.BrightGreen("Successfully").Italic().String() + " imported bookmarks from git\n").
		Flush()

	return nil
}

// Browser imports bookmarks from a supported browser.
func Browser(r *db.SQLiteRepository) error {
	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	}))
	br, ok := getBrowser(selectBrowser(t))
	if !ok {
		return fmt.Errorf("%w", browser.ErrBrowserUnsupported)
	}
	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}
	// find bookmarks
	bs, err := br.Import(t, config.App.Force)
	if err != nil {
		return fmt.Errorf("browser %q: %w", br.Name(), err)
	}
	// clean and process found bookmarks
	if err := parseFoundInBrowser(t, r, bs); err != nil {
		return err
	}
	if bs.Len() == 0 {
		return nil
	}

	return IntoRepo(t, r, bs)
}

// Database imports bookmarks from a database.
func Database(srcDB *db.SQLiteRepository) error {
	destDB, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer destDB.Close()

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
	m.SetPreprocessor(func(b *bookmark.Bookmark) string {
		return bookmark.Oneline(b, color.DefaultColorScheme())
	})

	records, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bs := slice.New(records...)

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if _, err := deduplicate(f, destDB, bs); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Midln("no new bookmark found, skipping import").Flush()
			return nil
		}

		return err
	}

	if err := destDB.InsertMany(context.Background(), bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	success := color.BrightGreen("Successfully").Italic().String()
	msg := fmt.Sprintf(success+" imported %d record/s from %s\n", bs.Len(), srcDB.Name())
	f.Clear().Success(msg).Flush()

	return nil
}

// IntoRepo import records into the database.
func IntoRepo(
	t *terminal.Term,
	r *db.SQLiteRepository,
	records *slice.Slice[bookmark.Bookmark],
) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if !config.App.Force {
		report := fmt.Sprintf("import %d records?", records.Len())
		if err := t.ConfirmErr(f.Row("\n").Question(report).String(), "y"); err != nil {
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
	success := color.BrightGreen("Successfully").Italic().String()
	msg := fmt.Sprintf(success+" imported %d record/s\n", records.Len())
	f.Clear().Success(msg).Flush()

	return nil
}

// FromBackup imports bookmarks from a backup.
func FromBackup(t *terminal.Term, f *frame.Frame, destDB, srcDB *db.SQLiteRepository) error {
	interruptFn := func(err error) {
		destDB.Close()
		srcDB.Close()
		sys.ErrAndExit(err)
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithMultiSelection(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithPreview(fmt.Sprintf("%s -n ./backup/%s {1}", config.App.Cmd, srcDB.Name())),
		menu.WithInterruptFn(interruptFn),
		menu.WithHeader("select record/s to import from '"+srcDB.Name()+"'", false),
	)
	defer t.CancelInterruptHandler()

	bookmarks, err := srcDB.All()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	cs := color.DefaultColorScheme()
	m.SetItems(bookmarks)
	m.SetPreprocessor(func(b *bookmark.Bookmark) string {
		return bookmark.Oneline(b, cs)
	})

	items, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	bs := slice.New(items...)

	if _, err := deduplicate(f, destDB, bs); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Clear().Row("\n").Mid("no new bookmark found, skipping import\n").Flush()
			return nil
		}

		return err
	}

	return nil
}

// mergeRecords merges non-duplicates records into database.
func mergeRecords(f *frame.Frame, dbPath, repoPath string) error {
	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := exportFromGit(f.Clear(), repoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	bookmarks = deduplicatePtr(f.Clear(), r, bookmarks)

	records := slice.New[bookmark.Bookmark]()
	for _, b := range bookmarks {
		records.Push(b)
	}

	if err := r.InsertMany(context.Background(), records); err != nil {
		return fmt.Errorf("%w", err)
	}

	n := len(bookmarks)
	if n > 0 {
		f.Clear().
			Success(fmt.Sprintf("Imported %d records into %q\n", n, filepath.Base(dbPath))).
			Flush()
	}

	return nil
}

// intoDB import into database.
func intoDB(f *frame.Frame, dbPath, dbName, repoPath string) error {
	bookmarks, err := exportFromGit(f.Clear(), repoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	dbPath = filepath.Join(filepath.Dir(dbPath), dbName)
	r, err := db.Init(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}

	if err := r.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	records := slice.New[bookmark.Bookmark]()
	for _, b := range bookmarks {
		records.Push(b)
	}

	if err := r.InsertMany(context.Background(), records); err != nil {
		return fmt.Errorf("%w", err)
	}

	f.Clear().
		Success(fmt.Sprintf("Imported %d records into %q\n", len(bookmarks), filepath.Base(dbPath))).
		Flush()

	return nil
}

func selectRecords(f *frame.Frame, dbPath, repoPath string) error {
	bookmarks, err := exportFromGit(f.Clear(), repoPath)
	if err != nil {
		return err
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import", false),
		menu.WithInterruptFn(func(err error) { // build interrupt cleanup
			sys.ErrAndExit(err)
		}),
	)

	coso := make([]bookmark.Bookmark, 0, len(bookmarks))
	for _, b := range bookmarks {
		coso = append(coso, *b)
	}

	slices.SortFunc(coso, func(a, b bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})

	cs := color.DefaultColorScheme()

	m.SetItems(coso)
	m.SetPreprocessor(func(b *bookmark.Bookmark) string {
		return bookmark.Oneline(b, cs)
	})
	selected, err := m.Select()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bs := slice.New(selected...)
	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	debookmarks, err := deduplicate(f.Clear(), r, bs)
	if err != nil {
		return err
	}

	n := len(debookmarks)
	if n > 0 {
		f.Clear().
			Success(fmt.Sprintf("Imported %d records into %q\n", n, filepath.Base(dbPath))).
			Flush()
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
