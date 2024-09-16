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
	f := frame.New(frame.WithColorBorder(color.Gray))
	path := format.PaddedLine("path:", r.Cfg.Fullpath())
	records := format.PaddedLine("records:", GetRecordCount(r, r.Cfg.TableMain))
	deleted := format.PaddedLine("deleted:", GetRecordCount(r, r.Cfg.TableDeleted))

	return f.Header(color.Yellow(r.Cfg.Name).Bold().Italic().String()).
		Row(records).
		Row(deleted).
		Row(path).String()
}

// SummaryRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
func SummaryRecords(s string) string {
	f := filepath.Base(s)
	c := NewSQLiteCfg(filepath.Dir(s))
	c.SetName(f)
	r, _ := New(c)

	main := fmt.Sprintf("(main: %d, ", GetRecordCount(r, r.Cfg.TableMain))
	deleted := fmt.Sprintf("deleted: %d)", GetRecordCount(r, r.Cfg.TableDeleted))
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

	mainRecords := color.BrightPurple(GetRecordCount(r, r.Cfg.TableMain))
	delRecords := color.BrightPurple(GetRecordCount(r, r.Cfg.TableDeleted))
	records := g("main: ").String() + mainRecords.String()
	records += sep
	records += g("deleted: ").String() + delRecords.String()
	date := formatBackupDate(f)

	return date + sep + name + sep + records
}

// BackupDetail returns the details of a backup.
func BackupDetail(r *SQLiteRepository) string {
	backups, _ := GetBackups(r)

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header(color.BrightCyan("backup detail:").Bold().Italic().String())

	n := backups.Len()
	if n == 0 {
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
		f            = frame.New(frame.WithColorBorder(color.Gray))
		empty        = "n/a"
		backups, _   = GetBackups(r)
		backupsColor = color.BrightMagenta("backups").Bold().Italic()
		backupsInfo  = format.PaddedLine("found:", empty)
		lastBackup   = empty
	)

	n := backups.Len()

	if n > 0 {
		backupsInfo = format.PaddedLine("found:", strconv.Itoa(n)+" backups found")
		lastBackup = SummaryRecords(backups.Get(n - 1))
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

	return d + " " + t
}
