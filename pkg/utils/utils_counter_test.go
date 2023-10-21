package utils_test

import (
	"gomarks/pkg/utils"
	"reflect"
	"testing"
)

func TestCounter_Add(t *testing.T) {
	c := make(utils.Counter)
	c.Add("tag1,tag2,tag3")
	c.Add("tag2,tag3,tag4")
	c.Add("tag1,tag2,tag4")

	expected := map[string]int{
		"tag1": 2,
		"tag2": 3,
		"tag3": 2,
		"tag4": 2,
	}
	for tag, count := range expected {
		if c.GetCount(tag) != count {
			t.Errorf(
				"Counter does not match expected values for %s. Got: %d, Expected: %d",
				tag,
				c.GetCount(tag),
				count,
			)
		}
	}
}

func TestCounter_GetCount(t *testing.T) {
	c := make(utils.Counter)
	c.Add("tag1,tag2,tag3")
	c.Add("tag2,tag3,tag4")

	count := c.GetCount("tag2")
	expectedCount := 2
	if count != expectedCount {
		t.Errorf("GetCount returned %d, expected %d", count, expectedCount)
	}

	nonExistentCount := c.GetCount("tag5")
	expectedNonExistentCount := 0
	if nonExistentCount != expectedNonExistentCount {
		t.Errorf(
			"GetCount returned %d, expected %d for non-existent tag",
			nonExistentCount,
			expectedNonExistentCount,
		)
	}
}

func TestCounter_Remove(t *testing.T) {
	c := make(utils.Counter)
	c.Add("tag1,tag2,tag3")
	c.Add("tag2,tag3,tag4")

	c.Remove("tag3")
	if c.GetCount("tag3") != 0 {
		t.Errorf("Remove did not work as expected. Tag 'tag3' still exists.")
	}

	c.Remove("tag4")
	if c.GetCount("tag4") != 0 {
		t.Errorf("Remove did not work as expected. Tag 'tag4' still exists.")
	}
}

func TestCounter_ToStringSlice(t *testing.T) {
	c := make(utils.Counter)
	c.Add("tag1,tag2,tag3")
	c.Add("tag2,tag3,tag4")

	expected := []string{
		"tag1 (1)",
		"tag2 (2)",
		"tag3 (2)",
		"tag4 (1)",
	}
	result := c.ToStringSlice()
	if !reflect.DeepEqual(result, expected) {
		t.Errorf(
			"ToStringSlice result does not match expected values. Got: %v, Expected: %v",
			result,
			expected,
		)
	}
}
