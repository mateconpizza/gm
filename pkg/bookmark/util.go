package bookmark

import (
	"fmt"
	"strings"

	"github.com/haaag/gm/pkg/editor"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/scraper"
	"github.com/haaag/gm/pkg/terminal"
)

const (
	_indentation = 10
)

var c = format.Color

// HandleURL handles the URL
func HandleURL(args *[]string) string {
	urlPrompt := c("+ URL\t:").Blue().Bold().String()

	if len(*args) > 0 {
		url := (*args)[0]
		*args = (*args)[1:]
		url = strings.TrimRight(url, "\n")
		fmt.Println(urlPrompt, url)
		return url
	}

	urlPrompt += c("\n > ").Orange().Bold().String()
	return terminal.ReadInput(urlPrompt)
}

// HandleTags handles the tags
func HandleTags(args *[]string) string {
	tagsPrompt := c("+ Tags\t:").Purple().Bold().String()

	if len(*args) > 0 {
		tags := (*args)[0]
		*args = (*args)[1:]
		tags = strings.TrimRight(tags, "\n")
		tags = strings.Join(strings.Fields(tags), ",")
		fmt.Println(tagsPrompt, tags)
		return tags
	}

	tagsPrompt += c(" (comma-separated)").Italic().Gray().String()
	tagsPrompt += c("\n > ").Orange().Bold().String()
	return terminal.ReadInput(tagsPrompt)
}

// HandleTitleAndDesc fetch and display title and description
func HandleTitleAndDesc(url string, minWidth int) (title, desc string) {
	var r strings.Builder
	sc := scraper.New(url)
	_ = sc.Scrape()
	r.WriteString(c("+ Title\t: ").Green().Bold().String())
	r.WriteString(format.SplitAndAlignString(sc.Title, minWidth, _indentation))
	r.WriteString(c("\n+ Desc\t: ").Yellow().Bold().String())
	r.WriteString(format.SplitAndAlignString(sc.Desc, minWidth, _indentation))
	fmt.Println(r.String())
	return sc.Title, sc.Desc
}

// ExtractIDsFromSlice extracts the IDs from a slice of bookmarks
func ExtractIDsFromSlice(bs *[]Bookmark) []int {
	ids := make([]int, 0, len(*bs))
	for _, b := range *bs {
		ids = append(ids, b.ID)
	}
	return ids
}

// ParseContent parses the provided content into a bookmark struct.
func ParseContent(content *[]string) *Bookmark {
	url := editor.ExtractBlock(content, "# URL:", "# Title:")
	title := editor.ExtractBlock(content, "# Title:", "# Tags:")
	tags := editor.ExtractBlock(content, "# Tags:", "# Description:")
	desc := editor.ExtractBlock(content, "# Description:", "# end")
	b := &Bookmark{URL: url, Title: title, Tags: format.ParseTags(tags), Desc: desc}
	checkTitleAndDesc(b)
	return b
}
