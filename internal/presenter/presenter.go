package presenter

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util/files"
	"github.com/haaag/gm/pkg/util/frame"
)

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

func paddingWithColor(label *color.Color, value any) string {
	const withColor = 32
	return fmt.Sprintf("%-*s %v", colorPadding(15, withColor), label, value)
}

// padding formats a label with right-aligned padding and appends a value.
func padding(label string, value any) string {
	const pad = 15
	return fmt.Sprintf("%-*s %v", pad, label, value)
}

// RepoSummary returns a summary of the repository.
func RepoSummary(r *repo.SQLiteRepository) string {
	f := frame.New(frame.WithColorBorder(color.Gray))
	path := padding("path:", r.Cfg.Path)
	records := padding("records:", r.GetMaxID(r.Cfg.GetTableMain()))
	deleted := padding("deleted:", r.GetMaxID(r.Cfg.GetTableDeleted()))

	return f.Header(color.Yellow(r.Cfg.Name).Bold().Italic().String()).
		Row(records).
		Row(deleted).
		Row(path).String()
}

// BackupsSummary returns a summary of the backups.
func BackupsSummary(r *repo.SQLiteRepository) string {
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
		lastBackup = RepoRecordSummary(r, backups[len(backups)-1])
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

func RepoRecordSummary(r *repo.SQLiteRepository, backup string) string {
	name := filepath.Base(backup)
	cfgCopy := *r.Cfg
	r.Cfg.SetName(name)
	r.Cfg.SetPath(filepath.Dir(backup))
	rep, _ := repo.New(r.Cfg)

	mainRecords := fmt.Sprintf("(main: %d, ", rep.GetMaxID(rep.Cfg.GetTableMain()))
	delRecords := fmt.Sprintf("deleted: %d)", rep.GetMaxID(rep.Cfg.GetTableDeleted()))
	records := color.Gray(mainRecords + delRecords).Italic()
	r.Cfg = &cfgCopy

	return name + " " + records.String()
}

// BackupDetail returns the details of a backup.
func BackupDetail(r *repo.SQLiteRepository) string {
	backups, _ := files.List(r.Cfg.BackupPath, r.Cfg.Name)

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header(color.BrightCyan("backup detail:").Bold().Italic().String())

	n := len(backups)
	if n == 0 {
		return f.Row(padding("found:", "n/a")).String()
	}

	for _, bk := range backups {
		f.Row(RepoRecordSummary(r, bk))
	}

	return f.String()
}

// Oneline formats a bookmark in a single line.
func Oneline(b *bookmark.Bookmark, maxWidth int) string {
	var sb strings.Builder
	const (
		idWithColor    = 18
		minTagsLen     = 34
		defaultTagsLen = 24
	)

	idLen := colorPadding(5, idWithColor)
	tagsLen := colorPadding(minTagsLen, defaultTagsLen)

	// calculate maximum length for url and tags based on total width
	urlLen := maxWidth - idLen - tagsLen

	// define template with formatted placeholders
	template := "%-*s%-*s %-*s\n"

	coloredID := color.BrightYellow(b.GetID()).Bold().String()
	shortURL := format.ShortenString(b.GetURL(), urlLen)
	colorURL := color.BrightWhite(shortURL).String()
	urlLen += len(colorURL) - len(shortURL)
	tagsColor := color.BrightCyan(b.GetTags()).Italic().String()
	result := fmt.Sprintf(template, idLen, coloredID, urlLen, colorURL, tagsLen, tagsColor)
	sb.WriteString(result)

	return sb.String()
}

// WithFrame formats a bookmark in a frame.
func WithFrame(b *bookmark.Bookmark, maxWidth int) string {
	n := maxWidth
	f := frame.New(
		frame.WithColorBorder(color.Gray),
		frame.WithMaxWidth(n),
	)

	// Indentation
	n -= len(f.Border.Row)

	// Split and add intendation
	descSplit := format.SplitIntoLines(b.GetDesc(), n)
	titleSplit := format.SplitIntoLines(b.GetTitle(), n)

	// Add color and style
	id := color.BrightYellow(b.GetID()).Bold().String()
	url := color.BrightMagenta(format.ShortenString(format.PrettifyURL(b.GetURL()), n)).
		String()
	title := color.ApplyMany(titleSplit, color.Cyan)
	desc := color.ApplyMany(descSplit, color.BrightWhite)
	tags := color.Gray(format.PrettifyTags(b.GetTags())).Italic().String()

	return f.Header(fmt.Sprintf("%s %s", id, url)).
		Mid(title...).Mid(desc...).
		Footer(tags).String()
}

// PrettyWithURLPath formats a bookmark with a URL formatted as a path
//
// Example: www.example.org • search • query.
func PrettyWithURLPath(b *bookmark.Bookmark, maxWidth int) string {
	const (
		bulletPoint = "\u2022" // •
		indentation = 8
		newLine     = 2
		spaces      = 6
	)

	var (
		sb        strings.Builder
		separator = strings.Repeat(" ", spaces) + "+"
		maxLine   = maxWidth - len(separator) - newLine
		title     = format.SplitAndAlignLines(b.GetTitle(), maxLine, indentation)
		prettyURL = format.URLPath(b.GetURL())
		shortURL  = format.ShortenString(prettyURL, maxLine)
		desc      = format.SplitAndAlignLines(b.GetDesc(), maxLine, indentation)
		id        = color.BrightWhite(b.GetID()).String()
		idSpace   = len(separator) - 1
		idPadding = strings.Repeat(" ", idSpace-len(strconv.Itoa(b.GetID())))
	)

	// Construct the formatted string
	sb.WriteString(
		fmt.Sprintf("%s%s%s %s\n", id, idPadding, bulletPoint, color.Purple(shortURL).String()),
	)
	sb.WriteString(color.Cyan(separator, title, "\n").String())
	sb.WriteString(color.Gray(separator, format.PrettifyTags(b.GetTags()), "\n").Italic().String())
	sb.WriteString(color.BrightWhite(separator, desc).String())

	return sb.String()
}

// WithFrameAndURLColor formats a bookmark with a given color.
func WithFrameAndURLColor(
	f *frame.Frame,
	b *bookmark.Bookmark,
	n int,
	c func(arg ...any) *color.Color,
) {
	const _midBulletPoint = "\u00b7"
	n -= len(f.Border.Row)

	titleSplit := format.SplitIntoLines(b.GetTitle(), n)
	idStr := color.BrightWhite(b.GetID()).Bold().String()

	url := c(format.ShortenString(format.PrettifyURL(b.GetURL()), n)).String()
	title := color.ApplyMany(titleSplit, color.Cyan)
	tags := color.Gray(format.PrettifyTags(b.GetTags())).Italic().String()

	f.Mid(fmt.Sprintf("%s %s %s", idStr, _midBulletPoint, url))
	f.Mid(title...).Mid(tags).Newline()
}
