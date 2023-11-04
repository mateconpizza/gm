package scrape

import (
	"testing"
)

func TestTitleAndDescription(t *testing.T) {
	url := "https://old.reddit.com"

	title, err := GetTitle(url)
	if err != nil {
		t.Fatalf("Error scraping: %v", err)
	}

	if title == "" {
		t.Error("Title is empty")
	}

	description, err := GetDescription(url)
	if err != nil {
		t.Fatalf("Error scraping: %v", err)
	}

	if description == "" {
		t.Error("Description is empty")
	}
}
