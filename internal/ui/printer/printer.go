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
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

func GitRepoTracked(f *frame.Frame, g *git.Manager) error {
	if len(g.Tracker.List) == 0 {
		return git.ErrGitNoTrackedRepos
	}

	dbFiles, err := files.Find(config.App.Path.Data, "*.db")
	if err != nil {
		return fmt.Errorf("finding db files: %w", err)
	}
	f.Header("Databases tracked in " + color.Orange("git\n").Italic().String()).Rowln().Flush()

	repos := make([]*git.GitRepository, 0, len(g.Tracker.List))
	for _, dbPath := range dbFiles {
		gr := g.NewRepo(dbPath)
		repos = append(repos, gr)
	}

	dimmer := color.Gray
	untracked := make([]*git.GitRepository, 0, len(repos))

	var sb strings.Builder
	for _, gr := range repos {
		sb.Reset()
		if !g.Tracker.Contains(gr) {
			untracked = append(untracked, gr)
			continue
		}

		sum := git.NewSummary()
		if err := handler.GitRepoStats(gr.DBPath, sum); err != nil {
			return fmt.Errorf("%w", err)
		}
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

		var t string
		if gpg.IsInitialized(g.RepoPath) {
			t = color.Cyan("gpg ").String()
		} else {
			t = color.Cyan("json ").String()
		}
		s := strings.TrimSpace(fmt.Sprintf("(%s)", strings.Join(parts, ", ")))
		sb.WriteString(txt.PaddedLine(gr.Name, t+dimmer(s).Italic().String()))

		f.Success(sb.String() + "\n").Flush()
	}

	for _, gr := range untracked {
		sb.Reset()
		sb.WriteString(txt.PaddedLine(gr.Name, dimmer("(not tracked)\n").Italic().String()))
		f.Error(sb.String()).Flush()
	}

	return nil
}

// RecordSlice prints the bookmarks in a frame format with the given colorscheme.
func RecordSlice(bs *slice.Slice[bookmark.Bookmark]) error {
	lastIdx := bs.Len() - 1
	bs.ForEachIdx(func(i int, b bookmark.Bookmark) {
		fmt.Print(bookmark.Frame(&b))
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
	bs.ForEach(func(b bookmark.Bookmark) {
		fmt.Print(bookmark.Oneline(&b))
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

	for _, fname := range fs {
		fmt.Print(db.RepoSummaryFromPath(f.Reset(), fname))
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
func RepoInfo(f *frame.Frame, p string, j bool) error {
	if err := locker.IsLocked(p); err != nil {
		fmt.Print(db.RepoSummaryFromPath(f, p+".enc"))
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

	fmt.Print(db.Info(f.Reset(), r))

	return nil
}
