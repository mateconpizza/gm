package scrape

import (
	"testing"
)

func TestTitleAndDescription(t *testing.T) {
	url := "https://old.reddit.com"

	result, err := TitleAndDescription(url)
	if err != nil {
		t.Fatalf("Error scraping: %v", err)
	}

	if result.Title == "" {
		t.Error("Title is empty")
	}

	if result.Description == "" {
		t.Error("Description is empty")
	}
}
