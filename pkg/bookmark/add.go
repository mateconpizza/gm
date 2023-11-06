package bookmark

import (
	"fmt"

	"gomarks/pkg/scrape"
)

func Add(url, tags string) (*Bookmark, error) {
	b := &Bookmark{
		URL:  url,
		Tags: parseTags(tags),
	}

	title, err := scrape.GetTitle(b.URL)
	if err != nil {
		return b, fmt.Errorf("%w: adding title", err)
	}

	b.setTitle(title)

	description, err := scrape.GetDescription(url)
	if err != nil {
		return b, fmt.Errorf("%w: adding description", err)
	}

	b.setDesc(description)

	return b, nil
}
