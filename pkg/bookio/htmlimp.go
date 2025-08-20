package bookio

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrNoHTMLFile     = errors.New("file is not an HTML file")
	ErrNoNetscapeFile = errors.New("file does not appear to be a valid Netscape bookmark file")
)

// BookmarkNetscape represents an individual bookmark.
type BookmarkNetscape struct {
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	Description  string    `json:"description"`
	Tags         []string  `json:"tags"`
	AddDate      time.Time `json:"add_date"`
	LastModified time.Time `json:"last_modified"`
}

// BookmarkNetscapeParser manage Bookmarks HTML file parsing.
type BookmarkNetscapeParser struct{}

// NewHTMLParser create a new instance of the Parser.
func NewHTMLParser() *BookmarkNetscapeParser {
	return &BookmarkNetscapeParser{}
}

// ParseHTML parse a HTMLNetscape bookmarks file and returns a bookmark's list.
func (bp *BookmarkNetscapeParser) ParseHTML(reader io.Reader) ([]BookmarkNetscape, error) {
	doc, err := html.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}

	// Validate it looks like a bookmark file
	if !bp.hasBookmarkElements(doc) {
		return nil, ErrNoNetscapeFile
	}

	var bs []BookmarkNetscape
	bp.parseNode(doc, &bs)
	return bs, nil
}

func (bp *BookmarkNetscapeParser) hasBookmarkElements(n *html.Node) bool {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "dt", "dd", "a":
			// Found bookmark-related elements
			return true
		case "h3":
			// Folder headers in bookmark files
			return true
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if bp.hasBookmarkElements(c) {
			return true
		}
	}
	return false
}

// parseNode recursively, look for elements <a> (Anchors) in the HTML.
func (bp *BookmarkNetscapeParser) parseNode(n *html.Node, bs *[]BookmarkNetscape) {
	if n.Type == html.ElementNode && n.Data == "a" {
		b := bp.parseBookmarkNode(n)
		if b != nil {
			*bs = append(*bs, *b)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		bp.parseNode(c, bs)
	}
}

// parseBookmarkNode extract information from a node <a>..
func (bp *BookmarkNetscapeParser) parseBookmarkNode(n *html.Node) *BookmarkNetscape {
	b := &BookmarkNetscape{}

	// Extract element <a> attributes
	for _, attr := range n.Attr {
		switch attr.Key {
		case "href":
			b.URL = attr.Val
		case "add_date":
			if timestamp, err := strconv.ParseInt(attr.Val, 10, 64); err == nil {
				b.AddDate = time.Unix(timestamp, 0)
			}
		case "last_modified":
			if timestamp, err := strconv.ParseInt(attr.Val, 10, 64); err == nil {
				b.LastModified = time.Unix(timestamp, 0)
			}
		case "tags":
			if attr.Val != "" {
				b.Tags = strings.Split(attr.Val, ",")
				// Clean white space
				for i, tag := range b.Tags {
					b.Tags[i] = strings.TrimSpace(tag)
				}
			}
		}
	}

	b.Title = bp.extractTextContent(n)

	// Validate that we have at least URL and title
	if b.URL == "" || b.Title == "" {
		return nil
	}

	// Find the next <DD> element for the description
	if n.Parent != nil {
		for sibling := n.Parent.NextSibling; sibling != nil; sibling = sibling.NextSibling {
			if sibling.Type == html.ElementNode && sibling.Data == "dd" {
				description := bp.extractTextContent(sibling)
				b.Description = description
				break
			}
		}
	}

	return b
}

// extractTextContent extract all the text content from a node.
func (bp *BookmarkNetscapeParser) extractTextContent(n *html.Node) string {
	var text strings.Builder
	bp.extractTextRecursive(n, &text)
	return strings.TrimSpace(text.String())
}

// extractTextRecursive recursivamente extrae texto de todos los nodos hijos.
func (bp *BookmarkNetscapeParser) extractTextRecursive(n *html.Node, text *strings.Builder) {
	if n.Type == html.TextNode {
		text.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		bp.extractTextRecursive(c, text)
	}
}

// FromNetscape converts a BookmarkNetscape to a Bookmark.
func FromNetscape(nb *BookmarkNetscape) *bookmark.Bookmark {
	if len(nb.Tags) == 0 {
		nb.Tags = append(nb.Tags, time.Now().Format("2006Jan02"))
	}

	return &bookmark.Bookmark{
		URL:       nb.URL,
		Tags:      bookmark.ParseTags(strings.Join(nb.Tags, ",")),
		Title:     nb.Title,
		Desc:      nb.Description,
		CreatedAt: nb.AddDate.Format(time.RFC3339),
		UpdatedAt: nb.LastModified.Format(time.RFC3339),
	}
}

// IsValidNetscapeFile checks if the file is a valid Netscape bookmark file
// by looking for the specific DOCTYPE declaration.
func IsValidNetscapeFile(file io.ReadSeeker) error {
	scanner := bufio.NewScanner(file)
	isNetscape := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "<!DOCTYPE NETSCAPE-Bookmark-file-1>") {
			isNetscape = true
			break
		}
	}

	// Reset file pointer
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("error resetting file pointer: %w", err)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	if !isNetscape {
		return ErrNoNetscapeFile
	}

	return nil
}
