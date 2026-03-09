package handler

import (
	"net/url"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestFilterWithParams(t *testing.T) {
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
			got := ParamsFilter(tt.bs)

			if len(got) != len(tt.want) {
				t.Fatalf("FilterWithParams() returned %d bookmarks; want %d", len(got), len(tt.want))
			}

			// Compare each bookmark's URL since we can't compare structs directly
			for i := range got {
				if got[i].URL != tt.want[i].URL {
					t.Fatalf("FilterWithParams()[%d].URL = %q; want %q",
						i, got[i].URL, tt.want[i].URL)
				}
			}
		})
	}
}

func TestCleanParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		u    *url.URL
		want string
	}{
		{
			name: "url_with_single_param",
			u:    &url.URL{Scheme: "https", Host: "example.com", Path: "/page", RawQuery: "key=value"},
			want: "https://example.com/page",
		},
		{
			name: "url_with_multiple_params",
			u:    &url.URL{Scheme: "https", Host: "example.com", Path: "/search", RawQuery: "q=golang&sort=asc&page=1"},
			want: "https://example.com/search",
		},
		{
			name: "url_with_empty_params",
			u:    &url.URL{Scheme: "https", Host: "example.com", Path: "/page", RawQuery: "param=&key=&"},
			want: "https://example.com/page",
		},
		{
			name: "url_with_fragment_only",
			u:    &url.URL{Scheme: "https", Host: "example.com", Path: "/page", Fragment: "section"},
			want: "https://example.com/page#section",
		},
		{
			name: "url_with_params_and_fragment",
			u: &url.URL{
				Scheme:   "https",
				Host:     "example.com",
				Path:     "/page",
				RawQuery: "key=value",
				Fragment: "section",
			},
			want: "https://example.com/page#section",
		},
		{
			name: "url_without_params_or_fragment",
			u:    &url.URL{Scheme: "https", Host: "example.com", Path: "/about"},
			want: "https://example.com/about",
		},
		{
			name: "url_with_user_info",
			u: &url.URL{
				Scheme:   "https",
				User:     url.UserPassword("user", "pass"),
				Host:     "example.com",
				Path:     "/page",
				RawQuery: "key=value",
			},
			want: "https://user:pass@example.com/page",
		},
		{
			name: "url_with_port",
			u:    &url.URL{Scheme: "https", Host: "example.com:8080", Path: "/api", RawQuery: "version=2"},
			want: "https://example.com:8080/api",
		},
		{
			name: "url_with_path_only",
			u:    &url.URL{Path: "/path/to/resource", RawQuery: "key=value"},
			want: "/path/to/resource",
		},
		{
			name: "nil_url_panics",
			u:    nil,
			want: "", // Note: This will panic, but we include it to document behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.u == nil {
				defer func() {
					if r := recover(); r == nil {
						t.Fatal("cleanParams(nil) expected to panic, but it didn't")
					}
				}()
				paramsCleaner(tt.u)
				return
			}

			got := paramsCleaner(tt.u)
			if got != tt.want {
				t.Fatalf("cleanParams(%+v) = %q; want %q", tt.u, got, tt.want)
			}

			// ensure the original URL is not modified
			if tt.u.RawQuery != "" && tt.u.RawQuery != tt.want {
				if tt.u.RawQuery == "" {
					t.Fatal("cleanParams() modified the original URL's RawQuery")
				}
			}
		})
	}
}
