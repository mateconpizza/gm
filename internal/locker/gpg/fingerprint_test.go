package gpg

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
)

func TestKeyResolver_Resolve(t *testing.T) {
	t.Parallel()

	errMockLoader := errors.New("mock loader failed")
	errMockLister := errors.New("mock lister failed")

	targetFP := &Fingerprint{Fingerprint: "0x1234567890ABCDEF"}
	otherFP := &Fingerprint{Fingerprint: "0x9999999999999999"}

	tests := []struct {
		name       string
		createFile bool
		loader     func(string) (string, error)
		lister     func(context.Context) ([]*Fingerprint, error)
		want       *Fingerprint
		wantErr    error
	}{
		{
			name:       "success_match_found",
			createFile: true,
			loader: func(path string) (string, error) {
				return targetFP.Fingerprint, nil
			},
			lister: func(ctx context.Context) ([]*Fingerprint, error) {
				return []*Fingerprint{otherFP, targetFP}, nil
			},
			want:    targetFP,
			wantErr: nil,
		},
		{
			name:       "not_initialized_file_missing",
			createFile: false,
			loader:     nil,
			lister:     nil,
			want:       nil,
			wantErr:    ErrNotInitialized,
		},
		{
			name:       "loader_error",
			createFile: true,
			loader: func(path string) (string, error) {
				return "", errMockLoader
			},
			lister:  nil,
			want:    nil,
			wantErr: errMockLoader,
		},
		{
			name:       "empty_recipient_error",
			createFile: true,
			loader: func(path string) (string, error) {
				return "", nil
			},
			lister:  nil,
			want:    nil,
			wantErr: ErrNoGPGRecipient,
		},
		{
			name:       "lister_error",
			createFile: true,
			loader: func(path string) (string, error) {
				return targetFP.Fingerprint, nil
			},
			lister: func(ctx context.Context) ([]*Fingerprint, error) {
				return nil, errMockLister
			},
			want:    nil,
			wantErr: errMockLister,
		},
		{
			name:       "empty_fingerprint_list",
			createFile: true,
			loader: func(path string) (string, error) {
				return targetFP.Fingerprint, nil
			},
			lister: func(ctx context.Context) ([]*Fingerprint, error) {
				return []*Fingerprint{}, nil
			},
			want:    nil,
			wantErr: ErrNoFingerprint,
		},
		{
			name:       "no_matching_fingerprint",
			createFile: true,
			loader: func(path string) (string, error) {
				return targetFP.Fingerprint, nil
			},
			lister: func(ctx context.Context) ([]*Fingerprint, error) {
				return []*Fingerprint{otherFP}, nil
			},
			want:    nil,
			wantErr: ErrNoFingerprint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			if tt.createFile {
				testutil.NewFile(t, tempDir, fingerprintIDFilename, []byte("test"))
			}

			opts := []KeyResolverOpt{}
			if tt.loader != nil {
				opts = append(opts, WithLoader(tt.loader))
			}
			if tt.lister != nil {
				opts = append(opts, WithLister(tt.lister))
			}

			resolver := NewKeyResolver(tempDir, opts...)
			got, err := resolver.Resolve(t.Context())

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Resolve() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Resolve() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Resolve() unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Resolve() = %v; want %v", got, tt.want)
			}
		})
	}
}
