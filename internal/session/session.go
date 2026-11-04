package session

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
	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

type (
	PromptFunc func(ctx context.Context, t *terminal.Term, repo string) (string, error)
	SessionOpt func(*Session)

	Session struct {
		prompt   PromptFunc // asks for a passphrase
		mu       sync.Mutex
		shared   string
		validate bool
		term     *terminal.Term
		writer   io.Writer
	}
)

func New(opts ...SessionOpt) *Session {
	es := &Session{
		prompt: promptNewPassphrase(os.Stdout),
		writer: os.Stdout,
		term:   terminal.New(),
	}
	for _, opt := range opts {
		opt(es)
	}
	return es
}

func (s *Session) Cipher(ctx context.Context, root, repo string, e enc.Encryption) (enc.Cipher, error) {
	switch e {
	case enc.GPG:
		return enc.NewGPG(ctx, root)

	case enc.AESGCM:
		p, err := s.passphraseAndValidate(ctx, root, repo)
		if err != nil {
			return nil, err
		}
		return enc.NewGCM(p), nil

	default:
		return &enc.PlainCipher{}, nil
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

	p, err := s.prompt(ctx, s.term, repo)
	if err != nil {
		return "", err // nothing is stored, so the next call asks again
	}
	s.shared = p

	return p, nil
}

func WithValidate() SessionOpt             { return func(s *Session) { s.validate = true } }
func WithPrompt(fn PromptFunc) SessionOpt  { return func(s *Session) { s.prompt = fn } }
func WithTerm(t *terminal.Term) SessionOpt { return func(s *Session) { s.term = t } }
func WithWriter(w io.Writer) SessionOpt    { return func(s *Session) { s.writer = w } }

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

// promptNewPassphrase asks twice, for repos that are about to be encrypted.
func promptNewPassphrase(w io.Writer) PromptFunc {
	return func(ctx context.Context, t *terminal.Term, repo string) (string, error) {
		fmt.Fprintf(w, "Password for %q: ", repo)
		p1, err := t.InputPassword(ctx)
		if err != nil {
			return "", err
		}

		defer t.ReplaceLine("")

		t.ReplaceLine(fmt.Sprintf("Confirm password for %q: ", repo))
		p2, err := t.InputPassword(ctx)
		if err != nil {
			return "", err
		}

		if p1 != p2 {
			return "", locker.ErrPassphraseMismatch
		}

		return p1, nil
	}
}

// promptPassphrase asks once, for repos that already exist (a typo just fails to decrypt).
func promptPassphrase(w io.Writer) PromptFunc {
	return func(ctx context.Context, t *terminal.Term, repo string) (string, error) {
		fmt.Fprintf(w, "Password for %q: ", repo)
		p, err := t.InputPassword(ctx)
		t.ReplaceLine("")

		return p, err
	}
}

func validatePassphrase(ctx context.Context, root, passphrase string) (string, error) {
	path, err := findSample(root, locker.Extension.String())
	if err != nil {
		return "", err
	}
	// no encrypted files found
	if path == "" {
		return passphrase, nil
	}
	if err := locker.IsValidPassphrase(ctx, path, passphrase); err != nil {
		return "", err
	}

	return passphrase, nil
}

// findSample returns one encrypted file from the repo, or "" if there are none
// yet.
func findSample(dir, suffix string) (string, error) {
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
