package bookmark

import (
	"errors"
	"testing"
)

func testSingleBookmark() *Bookmark {
	return &Bookmark{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,tag1,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01T12:00:00Z",
		LastVisit: "2023-01-01T12:00:00Z",
		Favorite:  true,
	}
}

func TestRecordIsValid(t *testing.T) {
	t.Parallel()

	validBookmark := testSingleBookmark()
	err := Validate(validBookmark)
	if err != nil {
		t.Fatalf("unexpected error validating valid bookmark: %v", err)
	}

	// URL err
	bookmarkWithoutURL := testSingleBookmark()
	bookmarkWithoutURL.URL = ""
	err = Validate(bookmarkWithoutURL)
	if err == nil {
		t.Fatalf("unexpected error validating valid bookmark: %v", err)
	}
	if !errors.Is(err, ErrURLEmpty) {
		t.Errorf("expected error to be %q, got %q", ErrURLEmpty.Error(), err.Error())
	}

	// Tags err
	bookmarkWithoutTags := testSingleBookmark()
	bookmarkWithoutTags.Tags = ""
	err = Validate(bookmarkWithoutTags)
	if err == nil {
		t.Fatalf("unexpected error validating valid bookmark: %v", err)
	}
	if !errors.Is(err, ErrTagsEmpty) {
		t.Errorf("expected error to be %q, got %q", ErrTagsEmpty.Error(), err.Error())
	}
}
