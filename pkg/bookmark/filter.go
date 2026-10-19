package bookmark

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
)

// TODO: unify `Deduplicate` and `Difference`
// Pass a simple function that defines de Key for the seen[Key]...

var ErrInvalidFilter = errors.New("invalid filter")

type filterDef struct {
	aliases []string
	match   func(b *Bookmark) bool
}

var filterDefs = map[string]filterDef{
	"active":   {[]string{"alive", "ok"}, func(b *Bookmark) bool { return b.IsActive }},
	"archived": {[]string{"wayback"}, func(b *Bookmark) bool { return b.ArchiveURL != "" }},
	"favorite": {[]string{"f", "fav", "favs", "favorites", "star", "starred"}, func(b *Bookmark) bool { return b.Favorite }},
	"inactive": {[]string{"dead", "broken", "down"}, func(b *Bookmark) bool { return !b.IsActive }},
	"notes":    {[]string{"annotated"}, func(b *Bookmark) bool { return strings.TrimSpace(b.Notes) != "" }},
	"unchecked": {[]string{"unc", "unverified"}, func(b *Bookmark) bool {
		_, ok := parseTime(b.LastStatusChecked)
		return !ok
	}},
	"untagged":  {[]string{"notags"}, func(b *Bookmark) bool { return strings.TrimSpace(b.Tags) == "" }},
	"unvisited": {[]string{"unv", "never", "unseen"}, func(b *Bookmark) bool { return b.VisitCount == 0 }},
	"visited":   {[]string{"seen"}, func(b *Bookmark) bool { return b.VisitCount > 0 }},
}

var (
	aliasToFilter = buildFilterLookup()

	// FilterSupported lists the canonical filter names.
	FilterSupported = slices.Sorted(maps.Keys(filterDefs))
)

func buildFilterLookup() map[string]string {
	m := make(map[string]string, len(filterDefs)*3)

	for name, d := range filterDefs {
		for _, a := range append([]string{name}, d.aliases...) {
			if prev, ok := m[a]; ok && prev != name {
				panic(fmt.Sprintf("filter alias %q used by both %q and %q", a, prev, name))
			}
			m[a] = name
		}
	}

	return m
}

// FilterBy keeps the bookmarks that match every filter in s.
// s is a comma-separated list of filter names or aliases, and a leading
// "!" negates one, e.g. "favorite,!unvisited". An empty s returns bs as is.
func FilterBy(s string, bs []*Bookmark) ([]*Bookmark, error) {
	if s == "" {
		return bs, nil
	}

	var preds []func(b *Bookmark) bool

	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		negate := strings.HasPrefix(part, "!")
		part = strings.TrimSpace(strings.TrimPrefix(part, "!"))

		key := strings.ToLower(part)
		key = strings.NewReplacer("_", "-", " ", "-").Replace(key)

		name, ok := aliasToFilter[key]
		if !ok {
			return nil, fmt.Errorf("%w %q: valid options: %s",
				ErrInvalidFilter, part, strings.Join(FilterSupported, ", "))
		}

		match := filterDefs[name].match
		if negate {
			preds = append(preds, func(b *Bookmark) bool { return !match(b) })
			continue
		}
		preds = append(preds, match)
	}

	if len(preds) == 0 {
		return bs, nil
	}

	return Filter(bs, func(b *Bookmark) bool {
		for _, p := range preds {
			if !p(b) {
				return false
			}
		}
		return true
	}), nil
}

// Deduplicate partitions bs into fresh and duplicate bookmarks.
func Deduplicate(bs, existing []*Bookmark) (fresh, duplicates []*Bookmark) {
	fresh = make([]*Bookmark, 0, len(bs))
	duplicates = make([]*Bookmark, 0, len(bs))
	seen := make(map[string]struct{}, len(existing))

	for _, b := range existing {
		if b == nil {
			continue
		}
		seen[b.URL] = struct{}{}
	}

	for _, b := range bs {
		if b == nil {
			continue
		}

		if _, ok := seen[b.URL]; ok {
			slog.Warn("deduplicate", "url", b.URL)
			duplicates = append(duplicates, b)
			continue
		}

		fresh = append(fresh, b)
	}

	return fresh, duplicates
}

// Difference returns bookmarks in b that are not in a.
func Difference(a, b []*Bookmark) []*Bookmark {
	bMap := make(map[string]struct{}, len(a))
	for _, bm := range a {
		if bm != nil {
			bMap[bm.Checksum] = struct{}{}
		}
	}

	var diff []*Bookmark
	for _, bm := range b {
		if bm == nil {
			continue
		}
		if _, exists := bMap[bm.Checksum]; !exists {
			diff = append(diff, bm)
		}
	}

	return diff
}

// Filter returns bookmarks matching predicate.
func Filter(bs []*Bookmark, predicate func(b *Bookmark) bool) []*Bookmark {
	filtered := make([]*Bookmark, 0, len(bs))
	for _, item := range bs {
		if predicate(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
