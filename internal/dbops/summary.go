package dbops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

type RepositoryMetadata interface {
	Metadata(key db.MetaKey) (string, error)
}

// SummaryRepoFromPath returns a summary of the repository.
func SummaryRepoFromPath(ctx context.Context, c *ui.Console, dbPath, backupPath string) string {
	f, p := c.Frame(), c.Palette()

	if base, found := strings.CutSuffix(dbPath, ".enc"); found {
		dbPath = base
		s := p.BrightMagenta.Wrap(filepath.Base(dbPath), p.Italic)

		e := "(locked)"
		if filepath.Base(dbPath) == application.MainDBName {
			e = "(main locked)"
		}

		return f.Mid(txt.PaddedLine(s, p.Gray.Wrap(e, p.Italic))).Ln().StringReset()
	}

	path := txt.PaddedLine("path:", files.CollapseHomeDir(dbPath))

	r, err := db.New(ctx, dbPath)
	if err != nil {
		return f.Row(path).StringReset()
	}
	defer r.Close()

	records := txt.PaddedLine("records:", r.Count(ctx, "bookmarks"))
	tags := txt.PaddedLine("tags:", r.Count(ctx, "tags"))
	name := p.Yellow.Wrap(r.Name(), p.Italic)

	if r.Name() == application.MainDBName {
		name = txt.PaddedLine(name, p.Gray.Wrap("(main)", p.Italic))
	}

	f.Headerln(name).Rowln(records).Rowln(tags)
	dbName := files.StripExts(r.Name())
	backups, _ := files.List(backupPath, "*_"+dbName+".db*")
	if len(backups) > 0 {
		f.Row(txt.PaddedLine("backups:", strconv.Itoa(len(backups)))).Ln()
	}

	return f.Rowln(path).StringReset()
}

// RepoInfo returns the repository info.
func RepoInfo(ctx context.Context, d *deps.Deps) (string, error) {
	var sb strings.Builder

	fn := func(s string, err error) error {
		if err != nil {
			return err
		}

		sb.WriteString(s)
		return nil
	}

	if err := fn(repository(ctx, d)); err != nil {
		return "", err
	}
	if err := fn(repoBackups(ctx, d)); err != nil {
		return "", err
	}
	if err := fn(repoBackupListDetail(ctx, d, false)); err != nil {
		return "", err
	}

	return sb.String(), nil
}

func relativeDays(createdAt string) string {
	parsed, err := time.Parse(db.TimeFormatSqlite, createdAt)
	if err != nil {
		return ""
	}

	return txt.RelativeTime(parsed.Format(txt.TimeLayout))
}

// repository returns a summary of the repository.
func repository(ctx context.Context, d *deps.Deps) (string, error) {
	r, err := d.Repository()
	if err != nil {
		return "", err
	}

	stats := db.NewStats()
	if err := r.Stats(ctx, stats); err != nil {
		return "", err
	}

	stats.Name = r.Name()

	p := d.Console().Palette()

	name := p.Bold.Sprint(r.Name())
	if r.Name() == application.MainDBName {
		name += p.Gray.Wrap(" (main) ", p.Italic)
	}

	f := d.Console().Frame()
	f.HeaderCln(p.Yellow, p.Yellow.Wrap(name, p.Italic)).
		Rowln(txt.PaddedLine("records:", stats.Bookmarks)).
		Rowln(txt.PaddedLine("tags:", stats.Tags))

	if stats.Favorites > 0 {
		f.Rowln(txt.PaddedLine("favorites:", stats.Favorites))
	}
	if stats.DeadLinks > 0 {
		f.Rowln(txt.PaddedLine("deadlinks:", stats.DeadLinks))
	}
	if stats.TotalVisits > 0 {
		f.Rowln(txt.PaddedLine("visits:", stats.TotalVisits))
	}

	f.Rowln(txt.PaddedLine("path:", files.CollapseHomeDir(r.Fullpath())))

	createdAt := createdAt(r, p)
	if createdAt != "" {
		f.Rowln(txt.PaddedLine("created:", createdAt))
	}

	return f.StringReset(), nil
}

// repoRecordsFromPath generates a summary of record counts for a given SQLite
// repository and bookmark.
//
//	repositoryName (n bookmarks n tags) | (locked)
func repoRecordsFromPath(ctx context.Context, c *ui.Console, fp string) string {
	p := c.Palette()
	if strings.HasSuffix(fp, ".enc") {
		s := strings.TrimSuffix(filepath.Base(fp), ".enc")
		return txt.PaddedLine(s, p.Gray.Wrap("(locked)", p.Italic))
	}

	r, err := db.New(ctx, fp)
	if err != nil {
		return p.BrightRed.Sprint("err", err.Error())
	}
	defer r.Close()

	stats := db.NewStats()
	if err := r.Stats(ctx, stats); err != nil {
		return p.BrightRed.Sprint("err")
	}

	main := fmt.Sprintf("(%d bookmarks %d tags)", stats.Bookmarks, stats.Tags)

	return txt.PaddedLine(r.Name(), p.Gray.Wrap(main, p.Italic))
}

// repoBackupListDetail returns the details of a backup.
func repoBackupListDetail(ctx context.Context, d *deps.Deps, complete bool) (string, error) {
	const maxItems = 3

	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	c, p := d.Console(), d.Console().Palette()
	fs, err := files.List(app.Path.Backup(), "*_"+app.DBBaseName()+".db*")
	if len(fs) == 0 {
		return "", nil
	}

	f := c.Frame()

	f.HeaderCln(p.BrightCyan, p.BrightCyan.Wrap("summary:", p.Italic))
	if err != nil {
		return f.Row(txt.PaddedLine("found:", "n/a\n")).String(), nil
	}

	if len(fs) > maxItems && !complete {
		f.Rowln(p.Gray.Sprintf("... %d more", len(fs)-maxItems))
		fs = fs[len(fs)-maxItems:]
	}

	for i := range fs {
		f.Rowln(formatBackupFn(ctx, p, fs[i], 0))
	}

	return f.StringReset(), nil
}

// repoBackups returns a summary of the backups.
//
// last, path and number of backups.
func repoBackups(ctx context.Context, d *deps.Deps) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	backupPath := app.Path.Backup()
	fs, err := Backups(ctx, d)
	if !errors.Is(err, db.ErrBackupNotFound) {
		return "", err
	}
	if len(fs) == 0 {
		return "", nil
	}

	p := d.Console().Palette()
	f := d.Console().Frame()

	f.HeaderCln(p.BrightMagenta, p.BrightMagenta.Wrap("backups:", p.Italic)).
		Rowln(txt.PaddedLine("path:", files.CollapseHomeDir(backupPath))).
		Rowln(txt.PaddedLine("found:", strconv.Itoa(len(fs))+" backups found"))

	last := fs[len(fs)-1]

	// check lock
	if err := locker.IsLocked(last); err != nil {
		// YYYYMMDD-HHMMSS_name.db (locked) today
		lastBackup := formatBackupFn(ctx, p, last, 10)
		lastDate := "-"
		parts := strings.Fields(lastBackup)
		if len(parts) >= 3 {
			lastDate = strings.Fields(lastBackup)[2]
		}
		return f.
			Rowln(txt.PaddedLine("last:", lastBackup)).
			Rowln(txt.PaddedLine("date:", p.BrightGreen.Wrap(p.Remover(lastDate), p.Italic))).
			StringReset(), nil
	}

	lastBackup, lastDate, err := lastBackupInfo(ctx, d, last)
	if err != nil {
		return "", fmt.Errorf("%w: %q", err, last)
	}

	return f.
		Rowln(txt.PaddedLine("last:", lastBackup)).
		Rowln(txt.PaddedLine("date:", p.BrightGreen.Wrap(lastDate, p.Italic))).
		StringReset(), nil
}

func createdAt(r RepositoryMetadata, p *ansi.Palette) string {
	createdAt, err := r.Metadata(db.MetaKeyCreatedAt)
	if err != nil {
		return ""
	}

	parsed, err := time.Parse(db.TimeFormatSqlite, createdAt)
	if err != nil {
		return ""
	}

	return createdAt + p.Gray.Sprintf(" (%s)", txt.RelativeTime(parsed.Format(txt.TimeLayout)))
}

func backupAt(r RepositoryMetadata) (string, error) {
	backupAt, err := r.Metadata(db.MetaKeyBackupAt)
	if err != nil {
		return "", err
	}

	parsed, err := time.Parse(db.TimeFormatSqlite, backupAt)
	if err != nil {
		return "", err
	}

	return txt.RelativeTime(parsed.Format(txt.TimeLayout)), nil
}

func lastBackupInfo(ctx context.Context, d *deps.Deps, path string) (filename, relative string, err error) {
	filename = repoRecordsFromPath(ctx, d.Console(), path)
	r, err := db.New(ctx, path)
	if err != nil {
		return "", "", err
	}
	defer r.Close()

	relative, err = backupAt(r)
	if err != nil {
		return filename, relative, nil
	}

	timestamp, _, _ := strings.Cut(filename, "_")
	return filename, txt.RelativeTime(timestamp), nil
}

func formatDatabaseFn(ctx context.Context, p *ansi.Palette, path string, pad int) string {
	name := filepath.Base(path)
	r, err := db.New(ctx, path)
	if err != nil {
		return name + ": " + err.Error()
	}
	defer r.Close()

	stats := db.NewStats()
	if err := r.Stats(ctx, stats); err != nil {
		return name + ": " + err.Error()
	}

	createdAt, err := r.Metadata(db.MetaKeyCreatedAt)
	if err != nil {
		createdAt = "err"
	}

	main := p.Gray.Sprintf(
		"(%d bookmarks %d tags)",
		stats.Bookmarks,
		stats.Tags,
	)

	createdAt = relativeDays(createdAt)
	if createdAt == "" {
		createdAt = "err"
	}

	name = files.StripExts(r.Name())
	return txt.PaddedLineWithPad(
		name,
		main+" "+createdAt,
		pad,
	)
}

// formatBackupFn generates a summary of record counts for a given
// SQLite repository.
//
//	repositoryName (main: n) or (locked) (time)
func formatBackupFn(ctx context.Context, p *ansi.Palette, path string, maxWidth int) string {
	name := filepath.Base(path)
	t, _, _ := strings.Cut(name, "_")
	bkTime := p.Gray.Wrap(txt.RelativeTime(t), p.Italic)
	if p.Remover(bkTime) == "invalid timestamp" {
		bkTime = "-"
	}

	if base, found := strings.CutSuffix(name, ".enc"); found {
		name = base + p.Gray.Wrap(" (locked) ", p.Italic)
		return name + bkTime
	}

	r, err := db.New(ctx, path)
	if err != nil {
		slog.Warn("creating repository from path", "path", path, "error", err)
		return p.Red.Sprint(err.Error())
	}
	defer r.Close()

	main := fmt.Sprintf("(%d bookmarks)", r.Count(ctx, "bookmarks"))

	return r.Name() + " " + p.Gray.Wrap(main, p.Italic) + " " + bkTime
}
