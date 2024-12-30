package blink

import (
	"reflect"
	"testing"
)

// generateChildren generates children for testing based in the JSON file.
func generateChildren() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"date_added":     "13379257043306561",
			"date_last_used": "0",
			"guid":           "8cb3b956-e4f1-4df5-bae9-b30275a29cab",
			"id":             "9",
			"meta_info": map[string]interface{}{
				"power_bookmark_meta": "",
			},
			"name": "Pass: The Standard Unix Password Manager",
			"type": "url",
			"url":  "https://www.passwordstore.org/",
		},
		map[string]interface{}{
			"guid": "1d5bff8a-426e-4982-b7d2-8110fe62e9ed",
			"id":   "10",
			"meta_info": map[string]interface{}{
				"power_bookmark_meta": "",
			},
			"name": "ExampleChad.net",
			"type": "url",
			"url":  "https://examplechad.net/",
		},
		map[string]interface{}{
			"children": []interface{}{
				map[string]interface{}{
					"date_added":     "13379257095727946",
					"date_last_used": "0",
					"guid":           "b86ee8a1-c719-41f7-a84f-b809252b3745",
					"id":             "11",
					"meta_info": map[string]interface{}{
						"power_bookmark_meta": "",
					},
					"name": "How to Check if a File or Directory Exists in Bash",
					"type": "url",
					"url":  "https://example.com/post/bash-check-if-file-exists/",
				},
			},
			"date_added":     "13379257105479987",
			"date_last_used": "0",
			"date_modified":  "13379257105480103",
			"guid":           "2ca1cf2e-84f8-4b09-9e15-93e10c721e36",
			"id":             "12",
			"name":           "bash",
			"type":           "folder",
		},
	}
}

var testBasicBookmarks = [][]string{
	{
		"Pass: The Standard Unix Password Manager",
		"https://www.passwordstore.org/",
		"testTag",
		"root",
	},
	{"ExampleChad.net", "https://examplechad.net/", "testTag", "root"},
	{
		"How to Check if a File or Directory Exists in Bash",
		"https://example.com/post/bash-check-if-file-exists/",
		"testTag",
		"bash",
	},
}

var testNoParentFolderBookmarks = [][]string{
	{
		"Pass: The Standard Unix Password Manager",
		"https://www.passwordstore.org/",
		"testTag2",
	},
	{"ExampleChad.net", "https://examplechad.net/", "testTag2"},
	{
		"How to Check if a File or Directory Exists in Bash",
		"https://example.com/post/bash-check-if-file-exists/",
		"testTag2",
	},
}

var testMissingFields = [][]string{
	{
		"",
		"",
		"testTag",
		"root",
	},
}

var testDuplicateNames = [][]string{
	{
		"Duplicate Name",
		"https://duplicate.example.com/",
		"testTag",
		"root",
	},
	{
		"Duplicate Name",
		"https://another-duplicate.example.com/",
		"testTag",
		"root",
	},
}

// generateMissingFields creates bookmarks with missing fields.
func generateMissingFields() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"name": "",
			"type": "url",
			"url":  "",
		},
	}
}

// generateDuplicateNames creates bookmarks with duplicate names.
func generateDuplicateNames() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"name": "Duplicate Name",
			"type": "url",
			"url":  "https://duplicate.example.com/",
		},
		map[string]interface{}{
			"name": "Duplicate Name",
			"type": "url",
			"url":  "https://another-duplicate.example.com/",
		},
	}
}

func TestTraverseBmFolder(t *testing.T) {
	tests := []struct {
		name                 string
		children             []interface{}
		uniqueTag            string
		parentName           string
		addParentFolderAsTag bool
		expected             [][]string
	}{
		{
			name:                 "Basic structure with URLs and a folder",
			children:             generateChildren(),
			uniqueTag:            "testTag",
			parentName:           "root",
			addParentFolderAsTag: true,
			expected:             testBasicBookmarks,
		},
		{
			name:                 "No parent folder tag",
			children:             generateChildren(),
			uniqueTag:            "testTag2",
			parentName:           "root",
			addParentFolderAsTag: false,
			expected:             testNoParentFolderBookmarks,
		},
		{
			name:                 "Missing fields",
			children:             generateMissingFields(),
			uniqueTag:            "testTag",
			parentName:           "root",
			addParentFolderAsTag: true,
			expected:             testMissingFields,
		},
		{
			name:                 "Duplicate names",
			children:             generateDuplicateNames(),
			uniqueTag:            "testTag",
			parentName:           "root",
			addParentFolderAsTag: true,
			expected:             testDuplicateNames,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := traverseBmFolder(
				tt.children,
				tt.uniqueTag,
				tt.parentName,
				tt.addParentFolderAsTag,
			)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected: %v, got: %v", tt.expected, result)
			}
		})
	}
}
