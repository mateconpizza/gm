package wayback

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// newTestWM returns a WaybackMachine whose client always answers with the
// given status and body.
func newTestWM(t *testing.T, status int, body string) *WaybackMachine {
	t.Helper()

	wm := New()
	wm.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		}),
	}

	return wm
}

func TestWaybackMachine_Snapshots(t *testing.T) {
	t.Parallel()

	const (
		hdr  = `["urlkey","timestamp","original"]`
		row1 = `["k","20260902090855","https://example.com/a"]`
		row2 = `["k","20260705074454","https://example.com/a"]`
	)

	tests := []struct {
		name       string
		status     int
		body       string
		want       []SnapshotInfo
		wantErr    error
		wantErrMsg string
	}{
		{
			name:   "multiple_rows",
			status: http.StatusOK,
			body:   `[` + hdr + `,` + row1 + `,` + row2 + `]`,
			want: []SnapshotInfo{
				{
					ArchiveURL:       "https://web.archive.org/web/20260902090855/https://example.com/a",
					ArchiveTimestamp: "20260902090855",
				},
				{
					ArchiveURL:       "https://web.archive.org/web/20260705074454/https://example.com/a",
					ArchiveTimestamp: "20260705074454",
				},
			},
		},
		{
			name:   "single_row",
			status: http.StatusOK,
			body:   `[` + hdr + `,` + row1 + `]`,
			want: []SnapshotInfo{{
				ArchiveURL:       "https://web.archive.org/web/20260902090855/https://example.com/a",
				ArchiveTimestamp: "20260902090855",
			}},
		},
		{
			name:   "short_rows_skipped",
			status: http.StatusOK,
			body:   `[` + hdr + `,["k","2026"],` + row1 + `,[]]`,
			want: []SnapshotInfo{{
				ArchiveURL:       "https://web.archive.org/web/20260902090855/https://example.com/a",
				ArchiveTimestamp: "20260902090855",
			}},
		},
		{
			// Documents current behavior: rows are filtered after the
			// len(raw) <= 1 check, so this returns an empty slice, not ErrNoSnapshots.
			name:   "only_short_rows",
			status: http.StatusOK,
			body:   `[` + hdr + `,["k","2026"]]`,
			want:   []SnapshotInfo{},
		},
		{name: "header_only", status: http.StatusOK, body: `[` + hdr + `]`, wantErr: ErrNoSnapshots},
		{name: "empty_array", status: http.StatusOK, body: `[]`, wantErr: ErrNoSnapshots},
		{name: "empty_body", status: http.StatusOK, body: ``, wantErrMsg: "unmarshal"},
		{name: "invalid_json", status: http.StatusOK, body: `not json`, wantErrMsg: "unmarshal"},
		{name: "status_429", status: http.StatusTooManyRequests, wantErr: ErrAPIRequestFail},
		{name: "status_498", status: 498, wantErr: ErrAPIRequestFail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wm := newTestWM(t, tt.status, tt.body)

			got, err := wm.Snapshots(t.Context(), "https://example.com/a")

			if tt.wantErr != nil || tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("Snapshots() expected error, got nil")
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("Snapshots() error = %v; want %v", err, tt.wantErr)
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Snapshots() error = %q; want it to contain %q", err, tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("Snapshots() unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("Snapshots() = %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestWaybackMachine_ClosestSnapshot(t *testing.T) {
	t.Parallel()

	const okBody = `{"archived_snapshots":{"closest":{"available":true,` +
		`"url":"http://web.archive.org/web/20260902090855/https://example.com/",` +
		`"timestamp":"20260902090855","status":"200"}}}`

	tests := []struct {
		name       string
		status     int
		body       string
		want       *SnapshotInfo
		wantErr    error
		wantErrMsg string
	}{
		{
			name:   "available",
			status: http.StatusOK,
			body:   okBody,
			want: &SnapshotInfo{
				ArchiveURL:       "http://web.archive.org/web/20260902090855/https://example.com/",
				ArchiveTimestamp: "20260902090855",
			},
		},
		{
			name:    "not_available",
			status:  http.StatusOK,
			body:    `{"archived_snapshots":{"closest":{"available":false}}}`,
			wantErr: ErrNoVersionAvailable,
		},
		{
			name:    "no_closest_key",
			status:  http.StatusOK,
			body:    `{"archived_snapshots":{}}`,
			wantErr: ErrNoVersionAvailable,
		},
		{name: "empty_object", status: http.StatusOK, body: `{}`, wantErr: ErrNoVersionAvailable},
		{name: "empty_body", status: http.StatusOK, body: ``, wantErrMsg: "decode"},
		{name: "invalid_json", status: http.StatusOK, body: `not json`, wantErrMsg: "decode"},
		{name: "status_429", status: http.StatusTooManyRequests, wantErr: ErrAPIRequestFail},
		{name: "status_500", status: http.StatusInternalServerError, wantErr: ErrAPIRequestFail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wm := newTestWM(t, tt.status, tt.body)

			got, err := wm.ClosestSnapshot(t.Context(), "https://example.com/")

			if tt.wantErr != nil || tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("ClosestSnapshot() expected error, got nil")
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("ClosestSnapshot() error = %v; want %v", err, tt.wantErr)
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("ClosestSnapshot() error = %q; want it to contain %q", err, tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("ClosestSnapshot() unexpected error: %v", err)
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("ClosestSnapshot() = %+v; want %+v", got, tt.want)
			}
		})
	}
}
