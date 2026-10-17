package bookmark

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/url"
	"slices"
	"strings"
	"time"
)

var ErrInvalidOption = errors.New("invalid option")

const NoDomain = "(no domain)"

var (
	// aliasToSort is the flat lookup: alias or canonical name -> canonical name.
	aliasToSort = buildAliasLookup()

	// SortSupported lists the canonical names, so it can't drift from the aliases.
	SortSupported = slices.Sorted(maps.Keys(sortAliases))
)

func buildAliasLookup() map[string]string {
	m := make(map[string]string, len(sortAliases)*4)

	add := func(name, canonical string) {
		if prev, ok := m[name]; ok && prev != canonical {
			panic(fmt.Sprintf("sort alias %q used by both %q and %q", name, prev, canonical))
		}
		m[name] = canonical
	}

	for canonical, aliases := range sortAliases {
		add(canonical, canonical)
		for _, a := range aliases {
			add(a, canonical)
		}
	}

	return m
}

// sortAliases maps each sort to the extra names it accepts.
// The keys are the canonical names.
var sortAliases = map[string][]string{
	"domain":    {"dom", "host", "site"},
	"favorite":  {"f", "fav", "favs", "favorites", "star", "starred"},
	"inactive":  {"dead", "broken", "down", "bad"},
	"newest":    {"new", "latest", "recent", "added"},
	"oldest":    {"old", "first"},
	"popular":   {"pop", "top", "visits", "vc", "most-visited"},
	"smart":     {"s", "best", "score", "rank"},
	"stale":     {"forgotten", "dormant", "cold"},
	"unchecked": {"unc", "unverified"},
	"unvisited": {"unv", "never", "unseen", "least-visited"},
	"updated":   {"up", "edited", "modified", "mod"},
	"visited":   {"last", "last-visit", "last-visited", "seen"},
	// "reverse":   {"rev", "r"},
}

// Score weights, tweak to taste.
const (
	wFav   = 1.0 // favorite
	wVisit = 0.6 // visit count (log scale)
	wRec   = 1.2 // recently visited
	wNew   = 0.6 // recently added, only if never visited
	wAlive = 0.3 // link responds with 2xx
	wDead  = 0.8 // link responds with 4xx/5xx

	recencyHalfLife = 21.0 // days
	maxVisitCount   = 100  // max visits
)

// Score ranks a bookmark, higher is better.
func Score(b *Bookmark, now time.Time) float64 {
	var score float64

	if b.Favorite {
		score += wFav
	}

	// log so heavy visit counts don't dominate
	if b.VisitCount > 0 {
		visits := min(b.VisitCount, maxVisitCount)
		score += wVisit * math.Log1p(float64(visits)) /
			math.Log1p(maxVisitCount)
	}

	// recency: last visit, or creation date if never visited
	if t, ok := parseTime(b.LastVisit); ok {
		score += wRec * decay(t, now)
	} else if t, ok := parseTime(b.CreatedAt); ok {
		score += wNew * decay(t, now)
	}

	// link health, unchecked (status 0) is neutral
	switch {
	case b.HTTPStatusCode >= 200 && b.HTTPStatusCode < 300:
		score += wAlive
	case b.HTTPStatusCode >= 400 || !b.IsActive:
		score -= wDead
	}

	return score
}

// SmartSort orders bookmarks by Score, highest first.
func SmartSort(bs []*Bookmark) { smartSortAt(bs, time.Now()) }

func smartSortAt(bs []*Bookmark, now time.Time) {
	type scored struct {
		b     *Bookmark
		score float64
		last  time.Time
	}

	// compute once per bookmark, not once per comparison
	items := make([]scored, len(bs))
	for i, b := range bs {
		last, _ := parseTime(b.LastVisit)
		items[i] = scored{b: b, score: Score(b, now), last: last}
	}

	slices.SortStableFunc(items, func(a, b scored) int {
		return cmp.Or(
			cmp.Compare(b.score, a.score),               // higher score first
			cmp.Compare(b.b.VisitCount, a.b.VisitCount), // more visits first
			b.last.Compare(a.last),                      // more recent first
			strings.Compare(a.b.URL, b.b.URL),
		)
	})

	for i := range items {
		bs[i] = items[i].b
	}
}

// Sort sorts bs in place and returns it.
func Sort(s string, bs []*Bookmark) ([]*Bookmark, error) {
	if s == "" {
		return bs, nil
	}

	var fn func(a, b *Bookmark) int

	switch normalizeSort(s) {
	case "domain":
		fn = func(a, b *Bookmark) int {
			return cmp.Or(
				strings.Compare(domainFromURL(a.URL), domainFromURL(b.URL)),
				strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)),
			)
		}

	case "favorite":
		fn = func(a, b *Bookmark) int {
			switch {
			case a.Favorite == b.Favorite:
				return 0
			case a.Favorite:
				return -1
			}
			return 1
		}

	case "inactive":
		fn = func(a, b *Bookmark) int {
			switch {
			case a.IsActive == b.IsActive:
				return cmp.Compare(b.HTTPStatusCode, a.HTTPStatusCode)
			case !a.IsActive:
				return -1
			}
			return 1
		}

	case "newest":
		fn = func(a, b *Bookmark) int {
			return cmp.Or(cmpTimeDesc(a.CreatedAt, b.CreatedAt), cmp.Compare(b.ID, a.ID))
		}

	case "oldest":
		fn = func(a, b *Bookmark) int {
			return cmp.Or(cmpTimeAsc(a.CreatedAt, b.CreatedAt), cmp.Compare(a.ID, b.ID))
		}

	case "popular":
		fn = func(a, b *Bookmark) int {
			return cmp.Or(
				cmp.Compare(b.VisitCount, a.VisitCount),
				cmpTimeDesc(a.LastVisit, b.LastVisit),
			)
		}

	case "reverse":
		fn = func(a, b *Bookmark) int {
			return cmp.Compare(b.ID, a.ID)
		}

	case "smart":
		SmartSort(bs)
		return bs, nil

	case "stale":
		fn = func(a, b *Bookmark) int {
			return cmp.Or(
				cmpTimeAsc(a.LastVisit, b.LastVisit),
				cmpTimeAsc(a.CreatedAt, b.CreatedAt),
			)
		}

	case "unchecked":
		fn = func(a, b *Bookmark) int {
			return cmpTimeAsc(a.LastStatusChecked, b.LastStatusChecked)
		}

	case "unvisited":
		fn = func(a, b *Bookmark) int {
			return cmp.Or(
				cmp.Compare(a.VisitCount, b.VisitCount),
				cmpTimeDesc(a.CreatedAt, b.CreatedAt),
			)
		}

	case "updated":
		fn = func(a, b *Bookmark) int {
			return cmpTimeDesc(a.UpdatedAt, b.UpdatedAt)
		}

	case "visited":
		fn = func(a, b *Bookmark) int {
			return cmp.Or(
				cmpTimeDesc(a.LastVisit, b.LastVisit),
				cmp.Compare(b.VisitCount, a.VisitCount),
			)
		}

	default:
		return nil, fmt.Errorf("%w %q: valid options: %s",
			ErrInvalidOption,
			s, strings.Join(SortSupported, ", "),
		)
	}

	slices.SortStableFunc(bs, fn)

	return bs, nil
}

// SmartFavorites returns the top favorites by Score. A limit <= 0 means no
// limit. The input slice is not modified.
func SmartFavorites(bs []*Bookmark, limit int) []*Bookmark {
	favs := make([]*Bookmark, 0, len(bs))
	for _, b := range bs {
		if b.Favorite {
			favs = append(favs, b)
		}
	}

	SmartSort(favs)

	if limit > 0 && len(favs) > limit {
		favs = favs[:limit]
	}

	return favs
}

func domainFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return NoDomain
	}

	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")

	if i := strings.Index(host, ":"); i != -1 {
		host = host[:i]
	}

	if host == "" {
		return NoDomain
	}

	return host
}

// decay returns 1.0 for "now" and 0.5 after recencyHalfLife days.
func decay(t, now time.Time) float64 {
	days := math.Max(0, now.Sub(t).Hours()/24) // future dates count as now
	return math.Pow(0.5, days/recencyHalfLife)
}

// cmpTimeAsc puts older first and missing/invalid times first.
func cmpTimeAsc(a, b string) int {
	ta, okA := parseTime(a)
	tb, okB := parseTime(b)

	switch {
	case okA && okB:
		return ta.Compare(tb)
	case okA:
		return 1
	case okB:
		return -1
	}
	return 0
}

// cmpTimeDesc puts newer first and missing/invalid times last.
func cmpTimeDesc(a, b string) int {
	ta, okA := parseTime(a)
	tb, okB := parseTime(b)

	switch {
	case okA && okB:
		return tb.Compare(ta)
	case okA:
		return -1
	case okB:
		return 1
	}
	return 0
}

var timeLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseTime(s string) (time.Time, bool) {
	// FIX: ensure all timestamps are `RFC3339`
	// and simplify this function.
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// normalizeSort returns the canonical sort name for a name or alias.
// Unknown values are returned cleaned up, so the caller can reject them.
func normalizeSort(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("_", "-", " ", "-").Replace(s)

	if c, ok := aliasToSort[s]; ok {
		return c
	}

	return s
}
