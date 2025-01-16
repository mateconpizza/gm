package repo

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
)

// Summary returns a summary of the repository.
func Summary(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	path := format.PaddedLine("path:", r.Cfg.Fullpath())
	records := format.PaddedLine("records:", CountRecords(r, r.Cfg.Tables.Main))
	deleted := format.PaddedLine("deleted:", CountRecords(r, r.Cfg.Tables.Deleted))
	tags := format.PaddedLine("tags:", CountRecords(r, r.Cfg.Tables.Tags))

	return f.Header(color.Yellow(r.Cfg.Name).Bold().Italic().String()).
		Row(records).
		Row(deleted).
		Row(tags).
		Row(path).String()
}

// SummaryRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
func SummaryRecords(s string) string {
	f := filepath.Base(s)
	c := NewSQLiteCfg(filepath.Dir(s))
	c.SetName(f)
	r, _ := New(c)

	main := fmt.Sprintf("(main: %d, ", CountRecords(r, r.Cfg.Tables.Main))
	deleted := fmt.Sprintf("deleted: %d)", CountRecords(r, r.Cfg.Tables.Deleted))
	records := color.Gray(main + deleted).Italic()
	date := formatBackupDate(f)

	return date + " " + records.String()
}

func SummaryMultiline(s string) string {
	g := color.BrightGray
	f := filepath.Base(s)
	c := NewSQLiteCfg(filepath.Dir(s))
	c.SetName(f)
	r, _ := New(c)

	sep := color.BrightGray("", format.MidBulletPoint, "").Bold().String()
	name := color.BrightYellow(strings.Split(s, "_")[2]).Italic().String()

	mainRecords := color.BrightPurple(CountRecords(r, r.Cfg.Tables.Main))
	delRecords := color.BrightPurple(CountRecords(r, r.Cfg.Tables.Deleted))
	records := g("main: ").String() + mainRecords.String()
	records += sep
	records += g("deleted: ").String() + delRecords.String()
	date := formatBackupDate(f)

	return date + sep + name + sep + records
}

// BackupDetail returns the details of a backup.
func BackupDetail(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header(color.BrightCyan("backup detail:").Bold().Italic().String())

	backups, err := Backups(r)
	if err != nil {
		return f.Row(format.PaddedLine("found:", "n/a")).String()
	}

	backups.ForEach(func(bk string) {
		f.Row(SummaryRecords(bk))
	})

	return f.String()
}

// BackupsSummary returns a summary of the backups.
func BackupsSummary(r *SQLiteRepository) string {
	var (
		f            = frame.New(frame.WithColorBorder(color.BrightGray))
		empty        = "n/a"
		backupsColor = color.BrightMagenta("backups").Bold().Italic()
		backupsInfo  = format.PaddedLine("found:", empty)
		lastBackup   = empty
	)

	var n int
	backups, err := Backups(r)
	if err != nil {
		n = 0
	} else {
		n = backups.Len()
	}

	if n > 0 {
		backupsInfo = format.PaddedLine("found:", strconv.Itoa(n)+" backups found")
		lastBackup = SummaryRecords(backups.Item(n - 1))
	}

	status := format.PaddedLine("status:", getBkStateColored(r.Cfg.Backup.Limit))
	keep := format.PaddedLine("max:", strconv.Itoa(r.Cfg.Backup.Limit)+" backups allowed")
	path := format.PaddedLine("path:", r.Cfg.Backup.Path)
	last := format.PaddedLine("last:", lastBackup)

	return f.Header(backupsColor.String()).
		Row(keep).
		Row(backupsInfo).
		Row(last).
		Row(path).
		Row(status).String()
}

// Info returns the repository info.
func Info(r *SQLiteRepository) string {
	s := Summary(r)
	s += BackupsSummary(r)
	s += BackupDetail(r)

	return s
}

// getBkStateColored returns a colored string with the backups status.
func getBkStateColored(n int) string {
	if n <= 0 {
		return color.BrightRed("disabled").String()
	}

	return color.BrightGreen("enabled").String()
}

func formatBackupDate(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) <= 2 {
		return s
	}

	d, t := parts[0], parts[1]

	t = strings.Replace(t, "-", ":", 1)

	return d + " " + t
}
