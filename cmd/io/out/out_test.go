package out

import (
	"reflect"
	"testing"

	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func TestParseCSVFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty_returns_default_header",
			input: "",
			want:  bookio.CSVDefaultHeader,
		},
		{
			name:  "whitespace_only_returns_default_header",
			input: "   ",
			want:  bookio.CSVDefaultHeader,
		},
		{
			name:  "all_returns_every_field",
			input: "all",
			want:  bookmark.Fields(),
		},
		{
			name:  "all_mixed_case_returns_every_field",
			input: "ALL",
			want:  bookmark.Fields(),
		},
		{
			name:  "all_anywhere_in_list_wins",
			input: "id,url,all",
			want:  bookmark.Fields(),
		},
		{
			name:  "normal_fields",
			input: "id,url,title",
			want:  []string{"id", "url", "title"},
		},
		{
			name:  "fields_with_spaces",
			input: "id, url , title",
			want:  []string{"id", "url", "title"},
		},
		{
			name:  "fields_upper_and_mixed_case",
			input: "ID,URL,Title",
			want:  []string{"id", "url", "title"},
		},
		{
			name:  "trailing_comma_trimmed",
			input: "id,url,",
			want:  []string{"id", "url"},
		},
		{
			name:  "leading_comma_trimmed",
			input: ",id,url",
			want:  []string{"id", "url"},
		},
		{
			name:  "duplicates_removed",
			input: "id,url,id,url",
			want:  []string{"id", "url"},
		},
		{
			name:  "single_valid_field",
			input: "url",
			want:  []string{"url"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseCSVFields(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCSVFields(%q)\n got  %v\n want %v", tt.input, got, tt.want)
			}
		})
	}
}
