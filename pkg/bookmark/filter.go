package bookmark

import "log/slog"

// TODO: unify `Deduplicate` and `Difference`
// Pass a simple function that defines de Key for the seen[Key]...

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
