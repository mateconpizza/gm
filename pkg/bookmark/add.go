package bookmark

import (
	"fmt"
	"gomarks/pkg/scrape"
)

func Add(url, tags string) (*Bookmark, error) {
	b := &Bookmark{
		URL:  url,
		Tags: tags,
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
