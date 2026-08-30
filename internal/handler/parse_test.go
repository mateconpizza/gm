package handler

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func Test_SaveNewBookmark(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		force      bool
		repoUnset  bool
		input      string
		wantErr    error
		wantErrMsg string
	}{
		{
			name:      "normal_prompt_yes",
			force:     false,
			repoUnset: false,
			input:     "yes\n",
			wantErr:   nil,
		},
		{
			name:      "normal_prompt_y_shortcut",
			force:     false,
			repoUnset: false,
			input:     "y\n",
			wantErr:   nil,
		},
		{
			name:      "force_flag_bypasses_prompt",
			force:     true,
			repoUnset: false,
			input:     "", // no input needed, should not prompt
			wantErr:   nil,
		},
		{
			name:      "prompt_no_aborts",
			force:     false,
			repoUnset: false,
			input:     "no\n",
			wantErr:   sys.ErrActionAborted,
		},
		{
			name:      "prompt_case_insensitive_n",
			force:     false,
			repoUnset: false,
			input:     "N\n",
			wantErr:   sys.ErrActionAborted,
		},
		{
			name:      "repo_not_found",
			force:     false,
			repoUnset: true,
			input:     "yes\n",
			wantErr:   db.ErrDBNotFound,
		},
		{
			name:       "choose_eof_error",
			force:      false,
			repoUnset:  false,
			input:      "", // EOF immediately
			wantErrMsg: "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := testutil.NewDeps(t)

			app, err := d.Application(t.Context())
			if err != nil {
				t.Fatalf("unexpected error setting up app: %v", err)
			}

			app.Flags.Force = tt.force

			// Conditionally setup repository to test missing repo boundary
			if !tt.repoUnset {
				r := testutil.NewInitializedEmptyDB(t, app.Path.DB())
				d.SetRepo(r)
			} else {
				d.SetRepo(nil)
			}

			c := d.Console()
			c.SetReader(strings.NewReader(tt.input))

			b := testutil.NewBookmark(t)

			err = saveNewBookmark(t.Context(), d, b)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("saveNewBookmark() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("saveNewBookmark() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("saveNewBookmark() expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("saveNewBookmark() expected error containing %q, got %v", tt.wantErrMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("saveNewBookmark() unexpected error: %v", err)
			}
		})
	}
}

type mockScraper struct {
	keywords string
	title    string
	desc     string
	favicon  string
	tags     []string
	err      error
}

func (m *mockScraper) Start(ctx context.Context) error { return m.err }
func (m *mockScraper) Keywords() (string, error)       { return m.keywords, m.err }
func (m *mockScraper) Title() (string, error)          { return m.title, m.err }
func (m *mockScraper) Desc() (string, error)           { return m.desc, m.err }
func (m *mockScraper) Favicon() (string, error)        { return m.favicon, m.err }
func (m *mockScraper) TagsRepo() ([]string, error)     { return m.tags, m.err }

type mockTerm struct {
	tags string
}

func (m *mockTerm) ChooseTags(p string, items map[string]int) string { return m.tags }
func (m *mockTerm) ClearLine(n int)                                  {}

func TestTagsFromArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		initialTags string
		scrapedTags string
		force       bool
		repoUnset   bool
		userInput   string
		wantTags    string
		wantErr     error
	}{
		{
			name:        "existing_tags_skip_scraper_and_prompt",
			initialTags: "go,cli",
			scrapedTags: "scraped,tags",
			force:       true,
			userInput:   "user,input\n",
			wantTags:    "go,cli",
			wantErr:     nil,
		},
		{
			name:        "scraper_provides_tags_skip_prompt",
			initialTags: "",
			scrapedTags: "web,dev",
			force:       true,
			userInput:   "user,input\n",
			wantTags:    "web,dev",
			wantErr:     nil,
		},
		{
			name:        "force_flag_uses_default_tag",
			initialTags: "",
			scrapedTags: "",
			force:       true,
			userInput:   "user,input\n",
			wantTags:    bookmark.DefaultTag,
			wantErr:     nil,
		},
		{
			name:        "user_prompt_success",
			initialTags: "",
			scrapedTags: "",
			force:       false,
			repoUnset:   false,
			userInput:   "manual,entry\n",
			wantTags:    "manual,entry",
			wantErr:     nil,
		},
		{
			name:        "user_prompt_empty_input",
			initialTags: "",
			scrapedTags: "",
			force:       false,
			repoUnset:   false,
			userInput:   "\n",
			wantTags:    bookmark.DefaultTag + ",",
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := testutil.NewDeps(t)

			app, err := d.Application(t.Context())
			if err != nil {
				t.Fatalf("unexpected app setup error: %v", err)
			}
			app.Flags.Force = tt.force

			r := testutil.NewInitializedEmptyDB(t, app.Path.DB())
			d.SetRepo(r)

			if tt.userInput != "" {
				d.Console().SetReader(strings.NewReader(tt.userInput))
			}

			sc := &mockScraper{keywords: tt.scrapedTags}
			b := &bookmarkTemp{tags: tt.initialTags}
			term := &mockTerm{tags: tt.wantTags}

			err = tagsFromArgs(t.Context(), d, term, sc, b)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("tagsFromArgs() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("tagsFromArgs() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("tagsFromArgs() unexpected error: %v", err)
			}

			expectedTags := tt.wantTags
			if expectedTags != "" && expectedTags != bookmark.DefaultTag {
				expectedTags = bookmark.ParseTags(expectedTags)
			}

			if b.tags != expectedTags {
				t.Errorf("tagsFromArgs() tags = %q; want %q", b.tags, expectedTags)
			}
		})
	}
}

func TestFetchTitleAndDesc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		initial bookmarkTemp
		scraper mockScraper
		want    bookmarkTemp
	}{
		{
			name: "title_provided_skips_scraping",
			initial: bookmarkTemp{
				title: "Existing Title",
				desc:  "Existing Desc",
			},
			scraper: mockScraper{
				title: "Scraped Title",
				desc:  "Scraped Desc",
			},
			want: bookmarkTemp{
				title: "Existing Title",
				desc:  "Existing Desc",
			},
		},
		{
			name:    "empty_bookmark_uses_scraper_data",
			initial: bookmarkTemp{},
			scraper: mockScraper{
				title:   "Scraped Title",
				desc:    "Scraped Desc",
				favicon: "favicon.ico",
			},
			want: bookmarkTemp{
				title:   "Scraped Title",
				desc:    "Scraped Desc",
				favicon: "favicon.ico",
				tags:    "",
			},
		},
		{
			name:    "scraper_keywords_used_when_tags_empty",
			initial: bookmarkTemp{},
			scraper: mockScraper{
				title:    "Some Title",
				keywords: "golang,cli,tool",
			},
			want: bookmarkTemp{
				title: "Some Title",
				tags:  "golang,cli,tool",
			},
		},
		{
			name:    "scraper_tagsrepo_overrides_keywords",
			initial: bookmarkTemp{},
			scraper: mockScraper{
				title:    "GitHub Repo",
				keywords: "ignore,these,keywords",
				tags:     []string{"github", "go", "test"},
			},
			want: bookmarkTemp{
				title: "GitHub Repo",
				tags:  "github,go,test",
			},
		},
		{
			name: "existing_tags_preserved",
			initial: bookmarkTemp{
				tags: "my,custom,tags",
			},
			scraper: mockScraper{
				title:    "Scraped Title",
				keywords: "scraped,tags",
				tags:     []string{"repo1", "repo2"},
			},
			want: bookmarkTemp{
				title: "Scraped Title",
				tags:  "my,custom,tags",
			},
		},
		{
			name:    "scraper_returns_empty_strings",
			initial: bookmarkTemp{},
			scraper: mockScraper{
				title:    "",
				desc:     "",
				favicon:  "",
				keywords: "",
				tags:     nil,
			},
			want: bookmarkTemp{
				title:   "",
				desc:    "",
				favicon: "",
				tags:    "",
			},
		},
		{
			name:    "long_text_does_not_panic",
			initial: bookmarkTemp{},
			scraper: mockScraper{
				title: strings.Repeat("A", 1000),
				desc:  strings.Repeat("B", 1000),
			},
			want: bookmarkTemp{
				title: strings.Repeat("A", 1000),
				desc:  strings.Repeat("B", 1000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := testutil.NewConsole(t, io.Discard)
			b := tt.initial
			sc := &tt.scraper

			fetchTitleAndDesc(t.Context(), c, sc, &b)

			if b.title != tt.want.title {
				t.Errorf("fetchTitleAndDesc() title = %q, want %q", b.title, tt.want.title)
			}
			if b.desc != tt.want.desc {
				t.Errorf("fetchTitleAndDesc() desc = %q, want %q", b.desc, tt.want.desc)
			}
			if b.favicon != tt.want.favicon {
				t.Errorf("fetchTitleAndDesc() favicon = %q, want %q", b.favicon, tt.want.favicon)
			}
			if b.tags != tt.want.tags {
				t.Errorf("fetchTitleAndDesc() tags = %q, want %q", b.tags, tt.want.tags)
			}
		})
	}
}
