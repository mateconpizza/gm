// Package bookio provides utilities to import and export bookmarks in
// various formats.
package bookio

import (
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

// ExportToNetscapeHTML exports bookmarks to Netscape HTML format.
func ExportToNetscapeHTML(bs []*bookmark.Bookmark, writer io.Writer) error {
	// Write HTML header
	_, err := writer.Write([]byte(`<!DOCTYPE NETSCAPE-Bookmark-file-1>
<!-- This is an automatically generated file.
     It will be read and overwritten.
     DO NOT EDIT! -->
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
`))
	if err != nil {
		return err
	}

	// Group bookmarks by tags to create folders
	tagGroups := groupBookmarksByTags(bs)

	// Write untagged bookmarks first
	if untagged, exists := tagGroups[""]; exists {
		for _, bookmark := range untagged {
			if err := writeBookmarkEntry(writer, bookmark); err != nil {
				return err
			}
		}
		delete(tagGroups, "")
	}

	// Write tagged bookmarks in folders
	for tag, taggedBookmarks := range tagGroups {
		if err := writeFolder(writer, tag, taggedBookmarks); err != nil {
			return err
		}
	}

	// Write HTML footer
	_, err = writer.Write([]byte("</DL><p>\n"))
	return err
}

// groupBookmarksByTags groups bookmarks by their primary tag (first tag).
func groupBookmarksByTags(bookmarks []*bookmark.Bookmark) map[string][]*bookmark.Bookmark {
	groups := make(map[string][]*bookmark.Bookmark)

	for _, bookmark := range bookmarks {
		var primaryTag string
		if bookmark.Tags != "" {
			tags := strings.Split(bookmark.Tags, ",")
			if len(tags) > 0 {
				primaryTag = strings.TrimSpace(tags[0])
			}
		}
		groups[primaryTag] = append(groups[primaryTag], bookmark)
	}

	return groups
}

// writeFolder writes a folder containing bookmarks.
func writeFolder(writer io.Writer, folderName string, bookmarks []*bookmark.Bookmark) error {
	// Write folder header
	folderHTML := fmt.Sprintf("    <DT><H3>%s</H3>\n    <DL><p>\n", html.EscapeString(folderName))
	if _, err := writer.Write([]byte(folderHTML)); err != nil {
		return err
	}

	// Write bookmarks in folder
	for _, bookmark := range bookmarks {
		if err := writeBookmarkEntry(writer, bookmark); err != nil {
			return err
		}
	}

	// Write folder footer
	_, err := writer.Write([]byte("    </DL><p>\n"))
	return err
}

// writeBookmarkEntry writes a single bookmark entry.
func writeBookmarkEntry(writer io.Writer, b *bookmark.Bookmark) error {
	// Parse created_at timestamp for ADD_DATE attribute
	addDate := ""
	if b.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, b.CreatedAt); err == nil {
			addDate = fmt.Sprintf(` ADD_DATE="%d"`, t.Unix())
		} else if t, err := time.Parse("2006-01-02 15:04:05", b.CreatedAt); err == nil {
			addDate = fmt.Sprintf(` ADD_DATE="%d"`, t.Unix())
		}
	}

	// Parse last_visit timestamp for LAST_VISIT attribute
	lastVisit := ""
	if b.LastVisit != "" {
		if t, err := time.Parse(time.RFC3339, b.LastVisit); err == nil {
			lastVisit = fmt.Sprintf(` LAST_VISIT="%d"`, t.Unix())
		} else if t, err := time.Parse("2006-01-02 15:04:05", b.LastVisit); err == nil {
			lastVisit = fmt.Sprintf(` LAST_VISIT="%d"`, t.Unix())
		}
	}

	// Add visit count if available
	visitCount := ""
	if b.VisitCount > 0 {
		visitCount = fmt.Sprintf(` VISIT_COUNT="%d"`, b.VisitCount)
	}

	// Add favicon if available
	icon := ""
	if b.FaviconURL != "" {
		icon = fmt.Sprintf(` ICON=%q`, html.EscapeString(b.FaviconURL))
	}

	// Add tags as a custom attribute (some browsers support this)
	tags := ""
	if b.Tags != "" {
		tags = fmt.Sprintf(` TAGS=%q`, html.EscapeString(b.Tags))
	}

	// Create the bookmark entry
	bookmarkHTML := fmt.Sprintf(
		"        <DT><A HREF=%q%s%s%s%s%s>%s</A>\n",
		html.EscapeString(b.URL),
		addDate,
		lastVisit,
		visitCount,
		icon,
		tags,
		html.EscapeString(b.Title),
	)

	if _, err := writer.Write([]byte(bookmarkHTML)); err != nil {
		return err
	}

	// Add description if available
	if b.Desc != "" {
		descHTML := fmt.Sprintf("        <DD>%s\n", html.EscapeString(b.Desc))
		if _, err := writer.Write([]byte(descHTML)); err != nil {
			return err
		}
	}

	return nil
}
