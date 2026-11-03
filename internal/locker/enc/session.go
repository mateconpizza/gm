package enc

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

type SessionOpt func(*Session)

type Session struct {
	prompt   func(ctx context.Context, repo string) (string, error) // asks for a passphrase
	mu       sync.Mutex
	shared   string
	validate bool

	writer io.Writer
	reader io.Reader
}

func NewSession(opts ...SessionOpt) *Session {
	es := &Session{
		prompt: promptNewPassphrase(os.Stdout),
		writer: os.Stdout,
	}
	for _, opt := range opts {
		opt(es)
	}
	return es
}

func (s *Session) Cipher(ctx context.Context, root, repo string, enc EncryptionType) (Cipher, error) {
	switch enc {
	case GPG:
		return NewGPG(ctx, root)

	case AESGCM:
		p, err := s.passphraseAndValidate(ctx, root, repo)
		if err != nil {
			return nil, err
		}
		return NewGCM(p), nil

	default:
		return &PlainCipher{}, nil
	}
}

func (s *Session) passphraseAndValidate(ctx context.Context, root, repo string) (string, error) {
	p, err := s.passphrase(ctx, repo)
	if err != nil {
		return "", err
	}

	if !s.validate {
		return p, nil
	}

	return validatePassphrase(ctx, root, p)
}

func (s *Session) passphrase(ctx context.Context, repo string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.shared != "" {
		return s.shared, nil
	}

	p, err := s.prompt(ctx, repo)
	if err != nil {
		return "", err // nothing is stored, so the next call asks again
	}
	s.shared = p

	return p, nil
}

func WithValidate() SessionOpt {
	return func(s *Session) { s.validate = true }
}

func WithPrompt(fn func(ctx context.Context, repo string) (string, error)) SessionOpt {
	return func(s *Session) { s.prompt = fn }
}

// WithPassphrase prompts once for an existing repository's passphrase.
func WithPassphrase() SessionOpt {
	return func(s *Session) {
		s.prompt = promptPassphrase(s.writer)
		s.validate = true
	}
}

// WithNewPassphrase prompts twice to confirm a new repository's passphrase.
func WithNewPassphrase() SessionOpt {
	return func(s *Session) {
		s.prompt = promptNewPassphrase(s.writer)
	}
}

func WithWriter(w io.Writer) SessionOpt {
	return func(s *Session) {
		s.writer = w
	}
}

func WithReader(r io.Reader) SessionOpt {
	return func(s *Session) {
		s.reader = r
	}
}

// promptNewPassphrase asks twice, for repos that are about to be encrypted.
func promptNewPassphrase(w io.Writer) func(ctx context.Context, repo string) (string, error) {
	return func(ctx context.Context, repo string) (string, error) {
		fmt.Fprintf(w, "Password for %q: ", repo)
		p1, err := terminal.Password(ctx)
		if err != nil {
			return "", err
		}

		replaceLine(w, fmt.Sprintf("Confirm password for %q: ", repo))
		p2, err := terminal.Password(ctx)
		if err != nil {
			return "", err
		}

		if p1 != p2 {
			return "", locker.ErrPassphraseMismatch
		}

		replaceLine(w, "")

		return p1, nil
	}
}

// promptPassphrase asks once, for repos that already exist (a typo just fails to decrypt).
func promptPassphrase(w io.Writer) func(ctx context.Context, repo string) (string, error) {
	return func(ctx context.Context, repo string) (string, error) {
		fmt.Fprintf(w, "Password for %q: ", repo)
		p, err := terminal.Password(ctx)
		replaceLine(w, "")

		return p, err
	}
}

func validatePassphrase(ctx context.Context, root, passphrase string) (string, error) {
	path, err := FindSample(root, locker.Extension.String())
	if err != nil {
		return "", err
	}
	if err := locker.IsValidPassphrase(ctx, path, passphrase); err != nil {
		return "", err
	}

	return passphrase, nil
}

// FindSample returns one encrypted file from the repo, or "" if there are none
// yet.
func FindSample(dir, suffix string) (string, error) {
	var found string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, suffix) {
			found = path
			return fs.SkipAll // stop at the first one
		}

		return nil
	})

	return found, err
}

// replaceLine replaces the current console line.
func replaceLine(w io.Writer, s string) {
	fmt.Fprintf(w, "\r\033[2K%s", s)
}
