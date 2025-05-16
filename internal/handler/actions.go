package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/haaag/rotato"
	"golang.org/x/sync/semaphore"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/bookmark/qr"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/encryptor"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

// openQR opens a QR-Code image in the system default image viewer.
func openQR(qrcode *qr.QRCode, b *Bookmark) error {
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

// QR handles creation, rendering or opening of QR-Codes.
func QR(bs *Slice, open bool) error {
	qrFn := func(b Bookmark) error {
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
func Copy(bs *Slice) error {
	var urls string
	bs.ForEach(func(b Bookmark) {
		urls += b.URL + "\n"
	})
	if err := sys.CopyClipboard(urls); err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	return nil
}

// Open opens the URLs in the browser for the bookmarks in the provided Slice.
func Open(bs *Slice) error {
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
	actionFn := func(b Bookmark) error {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			return fmt.Errorf("error acquiring semaphore: %w", err)
		}
		defer sem.Release(1)

		wg.Add(1)
		go func(b Bookmark) {
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

	return nil
}

// CheckStatus prints the status code of the bookmark URL.
func CheckStatus(bs *Slice) error {
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

// LockDB locks the database.
func LockDB(t *terminal.Term, rToLock string) error {
	slog.Debug("encrypting database", "name", config.App.DBName)
	if err := encryptor.IsEncrypted(rToLock); err != nil {
		return fmt.Errorf("%w", err)
	}
	if !files.Exists(rToLock) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, filepath.Base(rToLock))
	}
	f := frame.New(frame.WithColorBorder(color.Gray))
	if err := t.ConfirmErr(f.Question(fmt.Sprintf("Lock %q?", rToLock)).String(), "y"); err != nil {
		return fmt.Errorf("%w", err)
	}
	pass, err := passwordConfirm(t, f.Clear())
	if err != nil {
		return err
	}
	if err := encryptor.Lock(rToLock, pass); err != nil {
		return fmt.Errorf("%w", err)
	}
	success := color.BrightGreen("Successfully").Italic().String()
	fmt.Println(success + " database locked")

	return nil
}

// UnlockDB unlocks the database.
func UnlockDB(t *terminal.Term, rToUnlock string) error {
	if files.Exists(rToUnlock) {
		return fmt.Errorf("%w: %q", encryptor.ErrFileNotEncrypted, filepath.Base(rToUnlock))
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
	f.Question("Password: ").Flush()
	s, err := t.InputPassword()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if err := encryptor.Unlock(rToUnlock, s); err != nil {
		fmt.Println()
		return fmt.Errorf("%w", err)
	}
	success := color.BrightGreen("\nSuccessfully").Italic().String()
	fmt.Println(success + " database unlocked")

	return nil
}

func RemoveBackups(t *terminal.Term, f *frame.Frame, r *repo.SQLiteRepository) error {
	filesToRemove := slice.New[repo.SQLiteRepository]()
	backups, err := repo.Backups(r)
	if err != nil {
		if !errors.Is(err, repo.ErrBackupNotFound) {
			return fmt.Errorf("%w", err)
		}
		backups = slice.New[repo.SQLiteRepository]()
	}
	if backups.Len() == 0 {
		return nil
	}

	rm := color.BrightRed("remove").Bold().String()
	items := backups.Items()
	f.Clear().Mid(rm + " backups?")
	opt, err := t.Choose(f.String(), []string{"all", "no", "select"}, "n")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	switch strings.ToLower(opt) {
	case "n", "no":
		sk := color.BrightYellow("skipping").String()
		t.ReplaceLine(1, f.Clear().Warning(sk+" backup/s\n").Row().String())
	case "a", "all":
		filesToRemove.Set(items)
	case "s", "select":
		m := menu.New[repo.SQLiteRepository](
			menu.WithUseDefaults(),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithMultiSelection(),
			menu.WithHeader(fmt.Sprintf("select backup/s from %q", r.Name()), false),
			menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
		)
		selected, err := Selection(m, *items, repo.RepoSummaryRecords)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		t.ClearLine(1)
		filesToRemove.Append(selected...)
	}
	filesToRemove.Push(r)

	return rmDatabases(t, f.Clear(), filesToRemove)
}

func rmDatabases(t *terminal.Term, f *frame.Frame, dbs *slice.Slice[repo.SQLiteRepository]) error {
	s := color.BrightRed("removing").String()
	dbs.ForEachMut(func(r *repo.SQLiteRepository) {
		f.Mid(repo.RepoSummaryRecords(r)).Ln()
	})

	msg := s + " " + strconv.Itoa(dbs.Len()) + " items/s"
	if err := t.ConfirmErr(f.Row("\n").Question(msg+", continue?").String(), "n"); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp := rotato.New(
		rotato.WithMesg("removing database..."),
		rotato.WithMesgColor(rotato.ColorYellow),
	)
	sp.Start()

	rmRepo := func(r *repo.SQLiteRepository) error {
		if err := files.Remove(r.Cfg.Fullpath()); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	}

	if err := dbs.ForEachMutErr(rmRepo); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Done()
	t.ClearLine(format.CountLines(f.String()))
	s = color.BrightGreen("Successfully").Italic().String()
	f.Clear().Success(s + " " + strconv.Itoa(dbs.Len()) + " items/s removed\n").Flush()

	return nil
}

// SelectionRepo selects a repo.
func SelectionRepo(args []string) (string, error) {
	var repoPath string
	fs, err := filepath.Glob(config.App.Path.Data + "/*.db*")
	if err != nil {
		return repoPath, fmt.Errorf("%w", err)
	}
	if len(fs) == 0 {
		return repoPath, fmt.Errorf("%w", repo.ErrDBsNotFound)
	}
	if len(args) == 0 {
		repoPath, err = SelectItemFrom(fs, "select database to remove")
		if err != nil {
			return repoPath, fmt.Errorf("%w", err)
		}
	} else {
		repoName := args[0]
		for _, r := range fs {
			repoName = files.EnsureExt(repoName, ".db")
			s := filepath.Base(r)
			if s == repoName || s == repoName+".enc" {
				repoPath = r
				break
			}
		}
	}
	if repoPath == "" {
		return repoPath, fmt.Errorf("%w: %q", repo.ErrDBNotFound, args[0])
	}
	if !files.Exists(repoPath) {
		return repoPath, fmt.Errorf("%w: %q", repo.ErrDBNotFound, repoPath)
	}
	if err := encryptor.IsEncrypted(repoPath); err != nil {
		return repoPath, fmt.Errorf("%w", err)
	}

	return repoPath, nil
}
