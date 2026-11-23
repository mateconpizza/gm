package bookmark

import (
	"errors"
	"fmt"
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

// Deduplicate splits incoming into fresh bookmarks and duplicates of existing
// ones or of earlier incoming ones, matched by key.
func Deduplicate(incoming, existing []*Bookmark, key KeyFunc) (fresh, duplicates []*Bookmark) {
	fresh = make([]*Bookmark, 0, len(incoming))
	duplicates = make([]*Bookmark, 0)

	seen := make(map[string]struct{}, len(existing)+len(incoming))

	for _, b := range existing {
		if b != nil {
			seen[key(b)] = struct{}{}
		}
	}

	for _, b := range incoming {
		if b == nil {
			continue
		}

		k := key(b)
		if _, ok := seen[k]; ok {
			duplicates = append(duplicates, b)
			continue
		}

		seen[k] = struct{}{}
		fresh = append(fresh, b)
	}

	return fresh, duplicates
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

type KeyFunc func(*Bookmark) string

func KeyURL(b *Bookmark) string      { return b.URL }
func KeyChecksum(b *Bookmark) string { return b.Checksum }
func KeyTitle(b *Bookmark) string    { return strings.ToLower(strings.TrimSpace(b.Title)) }

func KeyDomain(b *Bookmark) string {
	d, err := b.Domain()
	if err != nil || d == "" {
		return DefaultDomain
	}
	return d
}

// DeduplicateURL separates incoming bookmarks by whether their URLs exist in
// existing.
func DeduplicateURL(existing, incoming []*Bookmark) (fresh, duplicates []*Bookmark) {
	return Partition(existing, incoming, KeyURL)
}

// ByChecksum returns bookmarks in b whose checksums do not occur in a.
func ByChecksum(a, b []*Bookmark) []*Bookmark {
	diff, _ := Partition(a, b, KeyChecksum)
	return diff
}

// Partition separates bookmarks in b by whether their keys exist in a.
func Partition(a, b []*Bookmark, key KeyFunc) (onlyInB, inBoth []*Bookmark) {
	seen := make(map[string]struct{}, len(a))

	for _, bm := range a {
		if bm != nil {
			seen[key(bm)] = struct{}{}
		}
	}

	for _, bm := range b {
		if bm == nil {
			continue
		}

		if _, ok := seen[key(bm)]; ok {
			inBoth = append(inBoth, bm)
			continue
		}

		onlyInB = append(onlyInB, bm)
	}

	return onlyInB, inBoth
}

func MarkImport(existing, incoming []*Bookmark, source string, key KeyFunc) (fresh, dups []*Bookmark) {
	fresh, dups = Deduplicate(incoming, existing, key)
	for _, b := range fresh {
		b.Meta.Source, b.Meta.IsNew = source, true
	}
	for _, b := range dups {
		b.Meta.Source, b.Meta.IsDup = source, true
	}
	return fresh, dups
}

// Removed in existing but gone from the source (e.g. deleted in the browser).
func Removed(existing, incoming []*Bookmark) []*Bookmark {
	removed, _ := Partition(incoming, existing, KeyURL)
	return removed
}

// Common bookmarks present in both lists (from existing's side).
func Common(existing, incoming []*Bookmark) []*Bookmark {
	_, both := Partition(incoming, existing, KeyURL)
	return both
}

// Changed same URL, different content, i.e. the bookmark should be updated.
func Changed(existing, incoming []*Bookmark) []*Bookmark {
	byURL := make(map[string]*Bookmark, len(existing))
	for _, b := range existing {
		if b != nil {
			byURL[b.URL] = b
		}
	}

	var out []*Bookmark
	for _, in := range incoming {
		if in == nil {
			continue
		}
		if old, ok := byURL[in.URL]; ok && old.Checksum != in.Checksum {
			in.ID = old.ID // keep identity for the update
			out = append(out, in)
		}
	}
	return out
}

// Unique returns bookmarks with distinct keys.
func Unique(bs []*Bookmark, key KeyFunc) []*Bookmark {
	fresh, _ := Deduplicate(bs, nil, key)
	return fresh
}

// GroupBy groups bookmarks by their keys.
func GroupBy(bs []*Bookmark, key KeyFunc) map[string][]*Bookmark {
	g := make(map[string][]*Bookmark)
	for _, b := range bs {
		if b != nil {
			k := key(b)
			g[k] = append(g[k], b)
		}
	}
	return g
}

// DuplicateGroups returns groups of bookmarks with duplicate keys.
func DuplicateGroups(bs []*Bookmark, key KeyFunc) [][]*Bookmark {
	var out [][]*Bookmark
	for _, g := range GroupBy(bs, key) {
		if len(g) > 1 {
			out = append(out, g)
		}
	}
	return out
}
