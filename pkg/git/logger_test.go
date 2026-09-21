package git

import "testing"

func TestParseLogLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		line   string
		want   LogEntry
		wantOK bool
	}{
		{
			name: "message_with_repo_and_status",
			line: "18c7434 [main] add wayback lookup (~mod:1)",
			want: LogEntry{
				Hash:   "18c7434",
				Repo:   "main",
				Mesg:   "add wayback lookup",
				Status: "(~mod:1)",
			},
			wantOK: true,
		},
		{
			name: "message_with_repo",
			line: "8a597a2 [main] commit changes",
			want: LogEntry{
				Hash: "8a597a2",
				Repo: "main",
				Mesg: "commit changes",
			},
			wantOK: true,
		},
		{
			name: "message_without_repo",
			line: "as982ks other",
			want: LogEntry{
				Hash: "as982ks",
				Mesg: "other",
			},
			wantOK: true,
		},
		{
			name: "message_with_trailing_whitespace",
			line: "d5aa3d7 [main] wayback lookup  (~mod:1)  ",
			want: LogEntry{
				Hash:   "d5aa3d7",
				Repo:   "main",
				Mesg:   "wayback lookup",
				Status: "(~mod:1)",
			},
			wantOK: true,
		},
		{
			name: "empty_line",
			line: "",
			want: LogEntry{
				Repo:   "stale-repo",
				Status: "(~mod:1)",
			},
			wantOK: false,
		},
		{
			name: "hash_only",
			line: "8a597a2",
			want: LogEntry{
				Repo:   "stale-repo",
				Status: "(~mod:1)",
			},
			wantOK: false,
		},
		{
			name: "status_reset",
			line: "8a597a2 [main] commit changes",
			want: LogEntry{
				Hash:   "8a597a2",
				Repo:   "main",
				Mesg:   "commit changes",
				Status: "",
			},
			wantOK: true,
		},
		{
			name: "repo_reset",
			line: "as982ks other",
			want: LogEntry{
				Hash: "as982ks",
				Mesg: "other",
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := LogEntry{
				Repo:   "stale-repo",
				Status: "(~mod:1)",
			}

			gotOK := parseLogLine(&e, tt.line)
			if gotOK != tt.wantOK {
				t.Fatalf("parseLogLine(%q) = %v; want %v", tt.line, gotOK, tt.wantOK)
			}

			if !gotOK {
				return
			}

			if e.Hash != tt.want.Hash {
				t.Errorf("Hash = %q; want %q", e.Hash, tt.want.Hash)
			}
			if e.Repo != tt.want.Repo {
				t.Errorf("Repo = %q; want %q", e.Repo, tt.want.Repo)
			}
			if e.Mesg != tt.want.Mesg {
				t.Errorf("Mesg = %q; want %q", e.Mesg, tt.want.Mesg)
			}
			if e.Status != tt.want.Status {
				t.Errorf("Status = %q; want %q", e.Status, tt.want.Status)
			}
		})
	}
}
