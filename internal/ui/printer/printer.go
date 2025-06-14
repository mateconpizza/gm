package printer

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

func GitRepoTracked(f *frame.Frame, t []string) error {
	if len(t) == 0 {
		return git.ErrGitNoTrackedRepos
	}

	f.Header("Tracked databases in git\n").Rowln().Flush()
	for _, r := range t {
		f.Success(r).Ln()
	}

	f.Flush()

	return nil
}

// RecordSlice prints the bookmarks in a frame format with the given colorscheme.
func RecordSlice(bs *slice.Slice[bookmark.Bookmark]) error {
	cs := color.DefaultColorScheme()

	lastIdx := bs.Len() - 1
	bs.ForEachIdx(func(i int, b bookmark.Bookmark) {
		fmt.Print(bookmark.Frame(&b, cs))
		if i != lastIdx {
			fmt.Println()
		}
	})

	return nil
}

// TagsList lists the tags.
func TagsList(p string) error {
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	tags, err := db.TagsList(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	fmt.Println(strings.Join(tags, "\n"))

	return nil
}

// Oneline formats the bookmarks in oneline.
func Oneline(bs *slice.Slice[bookmark.Bookmark]) error {
	cs := color.DefaultColorScheme()
	cs.Enabled = config.App.Color

	bs.ForEach(func(b bookmark.Bookmark) {
		fmt.Print(bookmark.Oneline(&b, cs))
	})

	return nil
}

// ByField prints the selected field.
func ByField(bs *slice.Slice[bookmark.Bookmark], f string) error {
	printer := func(b bookmark.Bookmark) error {
		f, err := b.Field(f)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(f)

		return nil
	}
	slog.Info("selected field", "field", f)

	if err := bs.ForEachErr(printer); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// DatabasesList lists the available databases.
func DatabasesList(p string) error {
	fs, err := files.FindByExtList(p, ".db", ".enc")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	n := len(fs)
	if n > 1 {
		nColor := color.BrightCyan(n).Bold().String()
		f.Header(nColor + " database/s found\n").Row("\n").Flush()
	}

	for _, f := range fs {
		fmt.Print(db.RepoSummaryFromPath(f))
	}

	return nil
}

// JSONRecordSlice formats the bookmarks in JSONRecordSlice.
func JSONRecordSlice(bs *slice.Slice[bookmark.Bookmark]) error {
	slog.Debug("formatting bookmarks in JSON", "count", bs.Len())
	var r []*bookmark.BookmarkJSON
	bs.ForEach(func(b bookmark.Bookmark) {
		r = append(r, b.ToJSON())
	})
	j, err := port.ToJSON(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	fmt.Println(string(j))

	return nil
}

// JSONTags formats the tags counter in JSON.
func JSONTags(p string) error {
	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	tags, err := db.TagsCounter(r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	j, err := port.ToJSON(tags)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(string(j))

	return nil
}

// RepoInfo prints the database info.
func RepoInfo(p string, j bool) error {
	if err := locker.IsLocked(p); err != nil {
		fmt.Print(db.RepoSummaryFromPath(p + ".enc"))
		return nil
	}

	r, err := db.New(p)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer r.Close()

	r.Cfg.BackupFiles, _ = r.ListBackups()
	if j {
		b, err := port.ToJSON(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		fmt.Println(string(b))

		return nil
	}

	fmt.Print(db.Info(r))

	return nil
}
