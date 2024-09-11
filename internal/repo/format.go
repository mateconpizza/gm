package repo

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/util/frame"
)

// Summary returns a summary of the repository.
func Summary(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	path := format.PaddedLine("path:", r.Cfg.Path)
	records := format.PaddedLine("records:", GetRecordCount(r, r.Cfg.GetTableMain()))
	deleted := format.PaddedLine("deleted:", GetRecordCount(r, r.Cfg.GetTableDeleted()))

	return f.Header(color.Yellow(r.Cfg.Name).Bold().Italic().String()).
		Row(records).
		Row(deleted).
		Row(path).String()
}

// SummaryRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
func SummaryRecords(r *SQLiteRepository, bk string) string {
	// FIX: redo
  path := filepath.Dir(bk)
  c := NewSQLiteCfg(path)
	c.SetName(filepath.Base(bk))
	rep, _ := New(c)

	main := fmt.Sprintf("(main: %d, ", GetRecordCount(rep, rep.Cfg.GetTableMain()))
	deleted := fmt.Sprintf("deleted: %d)", GetRecordCount(rep, rep.Cfg.GetTableDeleted()))
	records := color.Gray(main + deleted).Italic()

	date := GetModTime(c.Fullpath())

	return date + " " + records.String()
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
		f.Row(SummaryRecords(r, bk))
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
		lastBackup = SummaryRecords(r, backups.Get(n-1))
	}

	status := format.PaddedLine("status:", getBkStateColored(r.Cfg.MaxBackups))
	keep := format.PaddedLine("max:", strconv.Itoa(r.Cfg.MaxBackups)+" backups allowed")
	path := format.PaddedLine("path:", r.Cfg.BackupPath)
	last := format.PaddedLine("last:", lastBackup)

	return f.Header(backupsColor.String()).
		Row(keep).
		Row(backupsInfo).
		Row(last).
		Row(path).
		Row(status).String()
}

// getBkStateColored returns a colored string with the backups status.
func getBkStateColored(n int) string {
	if n <= 0 {
		return color.BrightRed("disabled").String()
	}

	return color.BrightGreen("enabled").String()
}
