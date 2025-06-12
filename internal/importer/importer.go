package importer

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/browser"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/menu"
	"github.com/mateconpizza/gm/internal/repo"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

// Git imports bookmarks from a git repository.
func Git(tmpPath, repoPath string, t *terminal.Term, f *frame.Frame) error {
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
		if err := loadGitRepo(tmpPath, repoName, t, f.Clear()); err != nil {
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
func Browser(r *repo.SQLiteRepository) error {
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
func Database(srcDB *repo.SQLiteRepository) error {
	destDB, err := repo.New(config.App.DBPath)
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
	if err := deduplicate(f, destDB, bs); err != nil {
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

// IntoRepo inserts records into the database.
func IntoRepo(
	t *terminal.Term,
	r *repo.SQLiteRepository,
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

func mergeRecords(f *frame.Frame, dbPath, repoPath string) error {
	r, err := repo.New(dbPath)
	if err != nil {
		return fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	bookmarks, err := parseGitRepo(f.Clear(), repoPath)
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

	f.Clear().
		Success(fmt.Sprintf("Imported %d records into %q\n", len(bookmarks), filepath.Base(dbPath))).
		Flush()

	return nil
}

func intoDB(f *frame.Frame, dbPath, dbName, repoPath string) error {
	bookmarks, err := parseGitRepo(f.Clear(), repoPath)
	if err != nil {
		return fmt.Errorf("importing bookmarks: %w", err)
	}

	dbPath = filepath.Join(filepath.Dir(dbPath), dbName)
	r, err := repo.Init(dbPath)
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
