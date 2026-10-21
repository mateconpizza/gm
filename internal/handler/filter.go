package handler

import (
	"cmp"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func WithStatusCode(code string) func([]*bookmark.Bookmark) []*bookmark.Bookmark {
	codes := strings.Split(strings.TrimSpace(code), ",")

	return func(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
		if code == "" {
			return bs
		}

		predicates := make([]func(*bookmark.Bookmark) bool, 0, len(codes))

		for _, code := range codes {
			predicate, ok := statusCodePredicate(strings.TrimSpace(code))
			if !ok {
				return nil
			}
			predicates = append(predicates, predicate)
		}

		result := bookmark.Filter(bs, func(b *bookmark.Bookmark) bool {
			for _, predicate := range predicates {
				if predicate(b) {
					return true
				}
			}
			return false
		})

		slices.SortFunc(result, func(a, b *bookmark.Bookmark) int {
			return cmp.Compare(a.ID, b.ID)
		})

		return result
	}
}

func WithoutSnapshots(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	return bookmark.Filter(bs, func(b *bookmark.Bookmark) bool {
		return b.ArchiveURL == ""
	})
}

func WithSnapshots(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	return bookmark.Filter(bs, func(b *bookmark.Bookmark) bool {
		return b.ArchiveURL != ""
	})
}

func WithNotes(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	return bookmark.Filter(bs, func(b *bookmark.Bookmark) bool {
		return b.Notes != ""
	})
}

func WithoutNotes(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	return bookmark.Filter(bs, func(b *bookmark.Bookmark) bool {
		return b.Notes == ""
	})
}

func WithURLParams(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	return bookmark.Filter(bs, func(b *bookmark.Bookmark) bool {
		u, err := url.Parse(b.URL)
		if err != nil {
			return false
		}
		if len(u.Query()) == 0 {
			return false
		}
		return true
	})
}

func statusCodePredicate(code string) (func(*bookmark.Bookmark) bool, bool) {
	switch {
	// exact status code: 200, 404, 503...
	case len(code) == 3:
		want, err := strconv.Atoi(code)
		if err != nil {
			return nil, false
		}

		return func(b *bookmark.Bookmark) bool {
			return b != nil && b.HTTPStatusCode == want
		}, true

	case len(code) == 1:
		class, err := strconv.Atoi(code)
		if err != nil {
			return nil, false
		}

		minCode := class * 100
		maxCode := minCode + 99

		return func(b *bookmark.Bookmark) bool {
			return b != nil &&
				b.HTTPStatusCode >= minCode &&
				b.HTTPStatusCode <= maxCode
		}, true

	default:
		return nil, false
	}
}
