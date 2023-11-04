package bookmark

import (
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

	title, err := scrape.GetTitle(b.URL)
	if err != nil {
		return b, err
	}

	b.setTitle(title)

	description, err := scrape.GetDescription(url)
	if err != nil {
		return b, err
	}

	b.setDesc(description)

	return b, nil
}
