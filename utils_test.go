package main

import (
	"database/sql"
	"testing"
)

func TestShortenString(t *testing.T) {
	input := "This is a long string"
	maxLength := 10
	expected := "This is..."
	result := shortenString(input, maxLength)

	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestFolderExists(t *testing.T) {
	testFolder := "/tmp/testfolder"
	exists := folderExists(testFolder)

	if exists {
		t.Errorf("Expected folder not to exist, but it does.")
	}
}

func TestExecuteCommand(t *testing.T) {
	menu := Menu{
		Command:   "echo",
		Arguments: []string{"Hello, World!"},
	}

	output, err := executeCommand(&menu, "")

	if err != nil {
		t.Errorf("Error executing command: %v", err)
	}

	if output != "Hello, World!\n" {
		t.Errorf("Unexpected output: %s", output)
	}
}

func TestToJSON(t *testing.T) {
	bookmarks := []Bookmark{
		{
			ID:         0,
			URL:        "http://example.com/book1",
			Title:      NullString{sql.NullString{String: "Book 1", Valid: false}},
			Tags:       "tag1,tag2,tag3",
			Desc:       NullString{sql.NullString{String: "Description 1", Valid: true}},
			Created_at: NullString{sql.NullString{String: "2023-01-01", Valid: true}},
		},
		{
			ID:         0,
			URL:        "http://example.com/book2",
			Title:      NullString{sql.NullString{String: "Book 2", Valid: false}},
			Tags:       "tag1,tag2,tag3",
			Desc:       NullString{sql.NullString{String: "Description 2", Valid: true}},
			Created_at: NullString{sql.NullString{String: "2023-01-02", Valid: true}},
		},
	}

	expectedJSON := `[
  {
    "URL": "http://example.com/book1",
    "Title": null,
    "Tags": "tag1,tag2,tag3",
    "Desc": "Description 1",
    "Created_at": "2023-01-01"
  },
  {
    "URL": "http://example.com/book2",
    "Title": null,
    "Tags": "tag1,tag2,tag3",
    "Desc": "Description 2",
    "Created_at": "2023-01-02"
  }
]`

	jsonString := toJSON(&bookmarks)

	if jsonString != expectedJSON {
		t.Errorf("Unexpected JSON output:\nExpected: %s\nActual: %s", expectedJSON, jsonString)
	}
}
