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
			"date_added":     "13379257074799501",
			"date_last_used": "0",
			"guid":           "1d5bff8a-426e-4982-b7d2-8110fe62e9ed",
			"id":             "10",
			"meta_info": map[string]interface{}{
				"power_bookmark_meta": "",
			},
			"name": "LandChad.net",
			"type": "url",
			"url":  "https://landchad.net/",
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
					"name": "How to Check if a File or Directory Exists in Bash | Linuxize",
					"type": "url",
					"url":  "https://linuxize.com/post/bash-check-if-file-exists/",
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
			expected: [][]string{
				{
					"Pass: The Standard Unix Password Manager",
					"https://www.passwordstore.org/",
					"testTag",
					"root",
				},
				{"LandChad.net", "https://landchad.net/", "testTag", "root"},
				{
					"How to Check if a File or Directory Exists in Bash | Linuxize",
					"https://linuxize.com/post/bash-check-if-file-exists/",
					"testTag",
					"bash",
				},
			},
		},
		{
			name:                 "No parent folder tag",
			children:             generateChildren(),
			uniqueTag:            "testTag2",
			parentName:           "root",
			addParentFolderAsTag: false,
			expected: [][]string{
				{
					"Pass: The Standard Unix Password Manager",
					"https://www.passwordstore.org/",
					"testTag2",
				},
				{"LandChad.net", "https://landchad.net/", "testTag2"},
				{
					"How to Check if a File or Directory Exists in Bash | Linuxize",
					"https://linuxize.com/post/bash-check-if-file-exists/",
					"testTag2",
				},
			},
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
