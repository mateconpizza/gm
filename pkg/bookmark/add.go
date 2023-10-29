package bookmark

import (
	"gomarks/pkg/scrape"
)

func Add(url, tags string) (*Bookmark, error) {
	b := &Bookmark{
		URL:  url,
		Tags: tags,
	}
	result, err := scrape.TitleAndDescription(b.URL)
	if err != nil {
		return b, err
	}
	b.setTitle(result.Title)
	b.setDesc(result.Description)
	return b, nil
}
