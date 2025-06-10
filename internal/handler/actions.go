package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mateconpizza/rotato"
	"golang.org/x/sync/semaphore"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/repo"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

// QR handles creation, rendering or opening of QR-Codes.
func QR(bs *slice.Slice[bookmark.Bookmark], open bool) error {
	qrFn := func(b bookmark.Bookmark) error {
		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return fmt.Errorf("%w", err)
		}

		if open {
			return openQR(qrcode, &b)
		}

		var sb strings.Builder
		sb.WriteString(b.Title + "\n")
		sb.WriteString(b.URL + "\n")
		sb.WriteString(qrcode.String())
		t := sb.String()
		fmt.Print(t)

		lines := len(strings.Split(t, "\n"))
		terminal.WaitForEnter()
		terminal.ClearLine(lines)

		return nil
	}

	if err := bs.ForEachErr(qrFn); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// Copy copies the URLs to the system clipboard.
func Copy(bs *slice.Slice[bookmark.Bookmark]) error {
	var urls string
	bs.ForEach(func(b bookmark.Bookmark) {
		urls += b.URL + "\n"
	})
	if err := sys.CopyClipboard(urls); err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	return nil
}

// Open opens the URLs in the browser for the bookmarks in the provided Slice.
func Open(r *repo.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	const maxGoroutines = 15
	// get user confirmation to procced
	o := color.BrightGreen("opening").Bold()
	s := fmt.Sprintf("%s %d bookmarks, continue?", o, bs.Len())
	if err := confirmUserLimit(bs.Len(), maxGoroutines, s); err != nil {
		return err
	}

	sp := rotato.New(
		rotato.WithMesg("opening bookmarks..."),
		rotato.WithMesgColor(rotato.ColorBrightGreen),
		rotato.WithSpinnerColor(rotato.ColorBrightGreen),
	)
	sp.Start()
	defer sp.Done()

	sem := semaphore.NewWeighted(maxGoroutines)
	var wg sync.WaitGroup
	errCh := make(chan error, bs.Len())
	actionFn := func(b bookmark.Bookmark) error {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}
		defer sem.Release(1)

		wg.Add(1)
		go func(b bookmark.Bookmark) {
			defer wg.Done()
			if err := sys.OpenInBrowser(b.URL); err != nil {
				errCh <- fmt.Errorf("open error: %w", err)
			}
		}(b)

		return nil
	}

	if err := bs.ForEachErr(actionFn); err != nil {
		return fmt.Errorf("%w", err)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}

	updateVisit := func(b bookmark.Bookmark) error {
		return r.UpdateVisitDateAndCount(context.Background(), &b)
	}
	if err := bs.ForEachErr(updateVisit); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// CheckStatus prints the status code of the bookmark URL.
func CheckStatus(bs *slice.Slice[bookmark.Bookmark]) error {
	n := bs.Len()
	if n == 0 {
		return repo.ErrRecordQueryNotProvided
	}

	const maxGoroutines = 15
	status := color.BrightGreen("status").Bold()
	q := fmt.Sprintf("checking %s of %d, continue?", status, n)
	if err := confirmUserLimit(n, maxGoroutines, q); err != nil {
		return sys.ErrActionAborted
	}

	f := frame.New(frame.WithColorBorder(color.BrightBlue))
	f.Header(fmt.Sprintf("checking %s of %d bookmarks\n", status, n)).Flush()
	if err := bookmark.Status(bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// LockRepo locks the database.
func LockRepo(t *terminal.Term, rToLock string) error {
	slog.Debug("locking database", "name", config.App.DBName)
	if err := locker.IsLocked(rToLock); err != nil {
		return fmt.Errorf("%w", err)
	}
	if !files.Exists(rToLock) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, filepath.Base(rToLock))
	}
	f := frame.New(frame.WithColorBorder(color.Gray))
	q := fmt.Sprintf("Lock %q?", filepath.Base(rToLock))
	if err := t.ConfirmErr(f.Question(q).String(), "y"); err != nil {
		if errors.Is(err, terminal.ErrActionAborted) {
			return nil
		}

		return fmt.Errorf("%w", err)
	}
	pass, err := passwordConfirm(t, f.Clear())
	if err != nil {
		return err
	}
	if err := locker.Lock(rToLock, pass); err != nil {
		return fmt.Errorf("%w", err)
	}
	success := color.BrightGreen("Successfully").Italic().String()
	fmt.Println(success + " database locked")

	return nil
}

// UnlockRepo unlocks the database.
func UnlockRepo(t *terminal.Term, rToUnlock string) error {
	if err := locker.IsLocked(rToUnlock); err == nil {
		return locker.ErrItemUnlocked
	}
	if !strings.HasSuffix(rToUnlock, ".enc") {
		rToUnlock += ".enc"
	}
	slog.Debug("unlocking database", "name", rToUnlock)
	if !files.Exists(rToUnlock) {
		s := filepath.Base(strings.TrimSuffix(rToUnlock, ".enc"))
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, s)
	}

	f := frame.New(frame.WithColorBorder(color.Gray))
	q := fmt.Sprintf("Unlock %q?", rToUnlock)
	if err := t.ConfirmErr(f.Question(q).String(), "y"); err != nil {
		return fmt.Errorf("%w", err)
	}
	f.Clear().Question("Password: ").Flush()
	s, err := t.InputPassword()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if err := locker.Unlock(rToUnlock, s); err != nil {
		fmt.Println()
		return fmt.Errorf("%w", err)
	}
	success := color.BrightGreen("\nSuccessfully").Italic().String()
	fmt.Println(success + " database unlocked")

	return nil
}

// openQR opens a QR-Code image in the system default image viewer.
func openQR(qrcode *qr.QRCode, b *bookmark.Bookmark) error {
	const maxLabelLen = 55
	var title string
	var burl string

	if err := qrcode.GenerateImg(config.App.Name); err != nil {
		return fmt.Errorf("%w", err)
	}

	title = format.Shorten(b.Title, maxLabelLen)
	if err := qrcode.Label(title, "top"); err != nil {
		return fmt.Errorf("%w: adding top label", err)
	}

	burl = format.Shorten(b.URL, maxLabelLen)
	if err := qrcode.Label(burl, "bottom"); err != nil {
		return fmt.Errorf("%w: adding bottom label", err)
	}

	if err := qrcode.Open(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// GitCommit commits the bookmarks to the git repo.
func GitCommit(actionMsg string) error {
	repoPath := config.App.Path.Git
	if !files.Exists(repoPath) {
		return nil
	}

	bookmarks, err := loadBookmarks(config.App.DBPath)
	if err != nil {
		return err
	}
	// FIX: what to do if no bookmarks found? return err? clean git repo?
	if len(bookmarks) == 0 {
		slog.Debug("no bookmarks found", "repo", repoPath)
		return nil
	}

	root := filepath.Join(repoPath, files.StripSuffixes(config.App.DBName))
	if err := exportBookmarks(repoPath, root, bookmarks); err != nil {
		return err
	}

	return commitIfChanged(repoPath, actionMsg)
}

func loadBookmarks(dbPath string) ([]*bookmark.Bookmark, error) {
	r, err := repo.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	defer r.Close()

	all, err := r.AllPtr()
	if err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}

	return all, nil
}

func exportBookmarks(repoPath, root string, bookmarks []*bookmark.Bookmark) error {
	if gpg.IsInitialized(repoPath) {
		if err := bookmark.StoreAsGPG(repoPath, bookmarks); err != nil {
			return fmt.Errorf("store as GPG: %w", err)
		}
	} else {
		if err := bookmark.ExportBookmarks(root, bookmarks); err != nil {
			return fmt.Errorf("export bookmarks: %w", err)
		}
		if err := diffDeletedBookmarks(root, bookmarks); err != nil {
			return fmt.Errorf("diff deleted: %w", err)
		}
	}
	return nil
}

// commitIfChanged commits the bookmarks to the git repo if there are changes.
func commitIfChanged(repoPath, actionMsg string) error {
	err := GitSummaryUpdate(repoPath)
	if err != nil {
		return err
	}

	// Check if any changes
	changed, _ := git.HasChanges(repoPath)
	if !changed {
		return git.ErrGitNothingToCommit
	}

	if err := git.AddAll(repoPath); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	status, err := git.Status(repoPath)
	if err != nil {
		status = ""
	}

	msg := fmt.Sprintf("[%s] %s %s", config.App.DBName, actionMsg, status)
	if err := git.CommitChanges(repoPath, msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

func GitSummaryUpdate(repoPath string) error {
	dbName := files.StripSuffixes(config.App.DBName)
	summaryPath := filepath.Join(repoPath, dbName, git.SummaryFileName)

	var summary *git.SyncGitSummary

	if !files.Exists(summaryPath) {
		// Create new summary with only RepoStats
		summary = git.NewSummary()
		if err := GitRepoStats(summary, repoPath); err != nil {
			return fmt.Errorf("creating repo stats: %w", err)
		}
	} else {
		// Load existing summary
		summary = git.NewSummary()
		if err := files.JSONRead(summaryPath, summary); err != nil {
			return fmt.Errorf("reading summary: %w", err)
		}
		// Update only RepoStats
		if err := GitRepoStats(summary, repoPath); err != nil {
			return fmt.Errorf("updating repo stats: %w", err)
		}
	}

	// Save updated or new summary
	if err := files.JSONWrite(summaryPath, summary, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}
	return nil
}
