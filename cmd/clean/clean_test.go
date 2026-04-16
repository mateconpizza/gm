package clean

import (
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func testBookmarksWithParameters(t *testing.T) []*bookmark.Bookmark {
	t.Helper()

	urls := []string{
		"https://example.com/products?utm_source=google&utm_medium=cpc&utm_campaign=spring_sale&fbclid=IwAR123abc456def",
		"https://blog.example.org/article?source=twitter&mc_cid=abc123&mc_eid=def456&_ga=2.123456789.987654321.123456789",
		"https://shop.example.net/item?id=12345&utm_content=buffer123&utm_term=keyword&_hsenc=p2ANqtz-abc123",
		"https://news.example.com/story?utm_source=feedburner&utm_medium=email&utm_campaign=Feed%3A+example&fb_ref=xyz789",
		"https://docs.example.io/guide?gclid=CjwKCAjwq42FBhB2EiwA8S8q1abc&utm_campaign=evergreen&utm_source=reddit&utm_medium=social",
		"https://app.example.co/login?redirect=/dashboard&_openstat=abc123;def456;ghi789&yclid=1234567890",
		"https://www.example.com/search?q=test&ref=sr_gw_1&pf_rd_r=ABC123&pf_rd_p=def456&pd_rd_wg=ghi789",
		"https://forum.example.org/topic/123?source=facebook&fb_action_ids=123456789&fb_action_types=og.likes",
		"https://store.example.com/checkout?session_id=abc123&_ga=GA1.2.123456789.123456789&_gac=1.123456789.123456789",
		"https://example.edu/course/view.php?id=123&utm_source=newsletter&utm_medium=email&utm_campaign=june2023&trk=profile_certification_title",
	}

	bs := testutil.BookmarkSlice(len(urls))
	for i := range bs {
		b := bs[i]
		b.URL = urls[i]
	}

	return bs
}

func TestFilterWithParams(t *testing.T) {
	// FIX: finish implementation
	_ = testBookmarksWithParameters(t)

	t.Parallel()

	tests := []struct {
		name string
		bs   []*bookmark.Bookmark
		want []*bookmark.Bookmark
	}{
		{
			name: "mixed_urls_some_with_params",
			bs: []*bookmark.Bookmark{
				{URL: "https://example.com/page?key=value"},
				{URL: "https://example.com/page"},
				{URL: "https://example.com/search?q=golang&sort=asc"},
				{URL: "https://example.com/about"},
			},
			want: []*bookmark.Bookmark{
				{URL: "https://example.com/page?key=value"},
				{URL: "https://example.com/search?q=golang&sort=asc"},
			},
		},
		{
			name: "all_urls_have_params",
			bs: []*bookmark.Bookmark{
				{URL: "https://example.com/page?key=value"},
				{URL: "https://example.com/search?q=golang"},
				{URL: "https://example.com/api?version=2&debug=true"},
			},
			want: []*bookmark.Bookmark{
				{URL: "https://example.com/page?key=value"},
				{URL: "https://example.com/search?q=golang"},
				{URL: "https://example.com/api?version=2&debug=true"},
			},
		},
		{
			name: "empty_params_edge_cases",
			bs: []*bookmark.Bookmark{
				{URL: "https://example.com/page?param="}, // empty value
				{URL: "https://example.com/page?key"},    // key with no value
				{URL: "https://example.com/page?=value"}, // empty key
				{URL: "https://example.com/page?"},       // trailing ?
				{URL: "https://example.com/page"},        // no params
			},
			want: []*bookmark.Bookmark{
				{URL: "https://example.com/page?param="},
				{URL: "https://example.com/page?key"},
				{URL: "https://example.com/page?=value"},
			},
		},
		{
			name: "invalid_urls_and_special_cases",
			bs: []*bookmark.Bookmark{
				{URL: "https://example.com/page?valid=param"},
				{URL: "not a valid url"},
				{URL: ""},
				{URL: "https://example.com/page"},          // no params
				{URL: "https://example.com/page#fragment"}, // fragment only
				{URL: "https://example.com/page?param=value#fragment"},
			},
			want: []*bookmark.Bookmark{
				{URL: "https://example.com/page?valid=param"},
				{URL: "https://example.com/page?param=value#fragment"},
			},
		},
		{
			name: "nil_and_empty_slice",
			bs:   nil,
			want: []*bookmark.Bookmark{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := WithURLParametersOnly(tt.bs)

			if len(got) != len(tt.want) {
				t.Fatalf("FilterWithParams() returned %d bookmarks; want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i].URL != tt.want[i].URL {
					t.Fatalf("FilterWithParams()[%d].URL = %q; want %q",
						i, got[i].URL, tt.want[i].URL)
				}
			}
		})
	}
}
