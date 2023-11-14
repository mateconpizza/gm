package util

import (
	"testing"
)

func TestFolderExists(t *testing.T) {
	testFolder := "/tmp/testfolder"
	exists := fileExists(testFolder)

	if exists {
		t.Errorf("Expected folder not to exist, but it does.")
	}
}

/* func TestExecuteCommand(t *testing.T) {
	m := menu.Menu{
		Command:   "echo",
		Arguments: []string{"Hello, World!"},
	}

	output, err := m.Run("")

	if err != nil {
		t.Errorf("Error executing command: %v", err)
	}

	if output != "Hello, World!" {
		t.Errorf("Unexpected output: %s", output)
	}
} */

/* func TestToJSON(t *testing.T) {
	bookmarks := []database.Bookmark{
		{
			ID:  0,
			URL: "http://example.com/book1",
			Title: database.NullString{
				NullString: sql.NullString{String: "Book 1", Valid: false},
			},
			Tags: "tag1,tag2,tag3",
			Desc: database.NullString{
				NullString: sql.NullString{String: "Description 1", Valid: true},
			},
			Created_at: "2023-01-01",
		},
		{
			ID:  0,
			URL: "http://example.com/book2",
			Title: database.NullString{
				NullString: sql.NullString{String: "Book 2", Valid: false},
			},
			Tags: "tag1,tag2,tag3",
			Desc: database.NullString{
				NullString: sql.NullString{String: "Description 2", Valid: true},
			},
			Created_at: "2023-01-01",
		},
	}

	expectedJSON := `[
  {
    "ID": 0,
    "URL": "http://example.com/book1",
    "Title": null,
    "Tags": "tag1,tag2,tag3",
    "Desc": "Description 1",
    "Created_at": "2023-01-01"
  },
  {
    "ID": 0,
    "URL": "http://example.com/book2",
    "Title": null,
    "Tags": "tag1,tag2,tag3",
    "Desc": "Description 2",
    "Created_at": "2023-01-02"
  }
]`

	jsonString := database.ToJSON(&bookmarks)

	if jsonString != expectedJSON {
		t.Errorf("Unexpected JSON output:\nExpected: %s\nActual: %s", expectedJSON, jsonString)
	}
} */

/* func TestPrettyFormatLine(t *testing.T) {
	label := "Test"
	testString := "This is a test string"
	expectedOutput := "Test                : This is a test string\n"
	s := u.PrettyFormatLine(label, testString)
	if s != expectedOutput {
		t.Errorf("Expected %s, but got %s", expectedOutput, s)
	}
} */
