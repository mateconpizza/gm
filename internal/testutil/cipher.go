// testutil/cipher.go
package testutil

import (
	"bytes"
	"context"
	"sync"

	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var _ enc.Cipher = (*FakeCipher)(nil)

// FakeCipher is an in-memory enc.Cipher for tests. It stores content as
// plain bytes keyed by path and can inject read/write errors.
type FakeCipher struct {
	ReadErr  error
	WriteErr error

	mu     sync.Mutex
	suffix string
	store  map[string][]byte
	writes int
}

// NewFakeCipher returns a FakeCipher that reports the given suffix.
func NewFakeCipher(suffix string) *FakeCipher {
	return &FakeCipher{suffix: suffix, store: map[string][]byte{}}
}

// Put seeds content at path without counting it as a write.
func (f *FakeCipher) Put(path string, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[path] = bytes.Clone(content)
}

// Get returns the content stored at path.
func (f *FakeCipher) Get(path string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.store[path]
	return b, ok
}

// Writes returns how many times Write succeeded or was attempted.
func (f *FakeCipher) Writes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes
}

func (f *FakeCipher) Write(_ context.Context, path string, content []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if f.WriteErr != nil {
		return f.WriteErr
	}
	f.store[path] = bytes.Clone(content)
	return nil
}

func (f *FakeCipher) Read(_ context.Context, path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ReadErr != nil {
		return nil, f.ReadErr
	}
	return bytes.Clone(f.store[path]), nil
}

func (f *FakeCipher) Path(*bookmark.Bookmark, string) (string, error) { return "", nil }
func (f *FakeCipher) Suffix() string                                  { return f.suffix }
