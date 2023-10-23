package utils

import (
	"fmt"
	"sort"
	"strings"
)

type Counter map[string]int

func (c Counter) Add(tags, sep string) {
	for _, tag := range strings.Split(tags, sep) {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			c[tag]++
		}
	}
}

func (c Counter) GetCount(item string) int {
	return c[item]
}

func (c Counter) Remove(item string) {
	delete(c, item)
}

func (c Counter) ToStringSlice() []string {
	var results []string
	for tag, count := range c {
		results = append(results, fmt.Sprintf("%s (%d)", tag, count))
	}
	sort.Strings(results)
	return results
}
