package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

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

func TestStreamLogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		r       io.Reader
		entry   *LogEntry
		wantErr error
	}{
		{
			name:    "normal_input",
			r:       strings.NewReader("e7e92be [main] initial commit (ok)\n"),
			entry:   NewLogEntry(),
			wantErr: nil,
		},
		{
			name:    "empty_input",
			r:       strings.NewReader(""),
			entry:   NewLogEntry(),
			wantErr: nil,
		},
		{
			name: "context_canceled_pre_scan",
			r:    strings.NewReader("e7e92be [main] message (ok)\n"),
			entry: &LogEntry{
				styler: &LogStyle{},
			},
			wantErr: context.Canceled,
		},
		{
			name:    "reader_error_returned",
			r:       iotest.ErrReader(io.ErrUnexpectedEOF),
			entry:   NewLogEntry(),
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "token_too_long_boundary",
			r:       strings.NewReader(strings.Repeat("a", bufio.MaxScanTokenSize+1)),
			entry:   NewLogEntry(),
			wantErr: bufio.ErrTooLong,
		},
		{
			name: "with_pre_processor_configured",
			r:    strings.NewReader("e7e92be [main] msg (ok)\n"),
			entry: &LogEntry{
				styler: &LogStyle{
					Hash:    func(s string) string { return s },
					Repo:    func(s string) string { return s },
					Message: func(s string) string { return s },
					Status:  func(s string) string { return s },
					Info:    func(s string) string { return s },
					PreProcessor: func(s string) string {
						return "MODIFIED: " + s
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			if errors.Is(tt.wantErr, context.Canceled) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}

			var w bytes.Buffer
			err := StreamLogs(ctx, tt.r, &w, tt.entry)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("StreamLogs() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("StreamLogs() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("StreamLogs() unexpected error: %v", err)
			}
		})
	}
}
