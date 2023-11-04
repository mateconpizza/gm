package bookmark

import (
	"fmt"
	"strings"

	"gomarks/pkg/scrape"
)

// convert: "tag1, tag3, tag tag"
// to:      "tag1,tag3,tag,tag,"
func parseTags(tags string) string {
	tags = strings.Join(strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	}), ",")

	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

func Add(url, tags string) (*Bookmark, error) {
	b := &Bookmark{
		URL:  url,
		Tags: parseTags(tags),
	}
	result, err := scrape.TitleAndDescription(b.URL)
	if err != nil {
		fmt.Printf("Error on %s: %s\n", b.URL, err)
		return b, nil
	}

	b.setTitle(result.Title)
	b.setDesc(result.Description)

	return b, nil
}
