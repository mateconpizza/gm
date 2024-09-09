package repo

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util/files"
	"github.com/haaag/gm/pkg/util/frame"
)

// Summary returns a summary of the repository.
func Summary(r *SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	path := padding("path:", r.Cfg.Path)
	records := padding("records:", r.GetMaxID(r.Cfg.GetTableMain()))
	deleted := padding("deleted:", r.GetMaxID(r.Cfg.GetTableDeleted()))

	return f.Header(color.Yellow(r.Cfg.Name).Bold().Italic().String()).
		Row(records).
		Row(deleted).
		Row(path).String()
}

// SummaryRecords generates a summary of record counts for a given SQLite
// repository and bookmark.
func SummaryRecords(r *SQLiteRepository, bk string) string {
	c := *r.Cfg
	name := filepath.Base(bk)

	c.SetName(name)
	c.SetPath(filepath.Dir(bk))
	rep, _ := New(&c)

	mainRecords := fmt.Sprintf("(main: %d, ", rep.GetMaxID(rep.Cfg.GetTableMain()))
	delRecords := fmt.Sprintf("deleted: %d)", rep.GetMaxID(rep.Cfg.GetTableDeleted()))
	records := color.Gray(mainRecords + delRecords).Italic()

	return name + " " + records.String()
}

// BackupDetail returns the details of a backup.
func BackupDetail(r *SQLiteRepository) string {
	backups, _ := files.List(r.Cfg.BackupPath, r.Cfg.Name)

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header(color.BrightCyan("backup detail:").Bold().Italic().String())

	n := len(backups)
	if n == 0 {
		return f.Row(padding("found:", "n/a")).String()
	}

	for _, bk := range backups {
		f.Row(SummaryRecords(r, bk))
	}

	return f.String()
}

// BackupsSummary returns a summary of the backups.
func BackupsSummary(r *SQLiteRepository) string {
	// FIX: paddingWithColor wont work when adding colors to var `backupsColor`
	var (
		f            = frame.New(frame.WithColorBorder(color.Gray))
		backups, _   = files.List(r.Cfg.BackupPath, r.Cfg.Name)
		backupsColor = color.BrightMagenta("backups:").Bold().Italic()
		backupsInfo  = paddingWithColor(backupsColor, "no backups found")
		lastBackup   = "n/a"
	)

	if len(backups) > 0 {
		backupsCount := color.BrightWhite(len(backups)).String()
		backupsInfo = paddingWithColor(backupsColor, backupsCount+" backups found")
		lastBackup = SummaryRecords(r, backups[len(backups)-1])
	}

	status := padding("status:", getBkStateColored(r.Cfg.MaxBackups))
	keep := padding("max:", strconv.Itoa(r.Cfg.MaxBackups)+" backups allowed")
	path := padding("path:", r.Cfg.BackupPath)
	last := padding("last:", lastBackup)

	return f.Mid(backupsInfo).
		Row(status).
		Row(keep).
		Row(last).
		Row(path).String()
}

// padding formats a label with right-aligned padding and appends a value.
func padding(label string, value any) string {
	const pad = 15
	return fmt.Sprintf("%-*s %v", pad, label, value)
}

func paddingWithColor(label *color.Color, value any) string {
	const withColor = 32
	return fmt.Sprintf("%-*s %v", colorPadding(15, withColor), label, value)
}

// colorPadding returns the padding for the colorized output.
func colorPadding(minVal, maxVal int) int {
	if terminal.Color {
		return maxVal
	}

	return minVal
}

// getBkStateColored returns a colored string with the backups status.
func getBkStateColored(n int) string {
	if n <= 0 {
		return color.BrightRed("disabled").String()
	}

	return color.BrightGreen("enabled").String()
}
