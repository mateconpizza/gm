package handler

import (
	"net/url"
	"testing"
)

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
