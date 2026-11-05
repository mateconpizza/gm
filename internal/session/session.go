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
	// PromptFunc asks for a passphrase.
	PromptFunc func(ctx context.Context, t *terminal.Term, repo string, confirm bool) (string, error)

	// Session manages passphrase acquisition, caching, and validation.
	Session struct {
		prompt       PromptFunc
		shared       string
		validation   bool
		confirmation bool

		mu     sync.Mutex
		term   *terminal.Term
		writer io.Writer
	}
)

// New returns a new Session with the provided options.
func New() *Session {
	return &Session{
		prompt: defaultPrompt(os.Stdout),
		term:   terminal.New(),
		writer: os.Stdout,
	}
}

// WithConfirmation sets whether passphrase confirmation is required.
func (s *Session) WithConfirmation(b bool) *Session {
	s.confirmation = b
	return s
}

// WithValidation sets whether passphrases are validated against encrypted
// files.
func (s *Session) WithValidation(b bool) *Session {
	s.validation = b
	return s
}

// WithPrompt sets a custom prompt function.
func (s *Session) WithPrompt(fn PromptFunc) *Session {
	if fn != nil {
		s.prompt = fn
	}
	return s
}

// WithTerm sets a custom terminal instance.
func (s *Session) WithTerm(t *terminal.Term) *Session {
	if t != nil {
		s.term = t
	}
	return s
}

// WithWriter sets a custom output writer.
func (s *Session) WithWriter(w io.Writer) *Session {
	if w != nil {
		s.writer = w
	}
	return s
}

// WithPassphrase sets a fixed passphrase, bypassing interactive prompting.
func (s *Session) WithPassphrase(passphrase string) *Session {
	s.shared = passphrase
	s.prompt = func(context.Context, *terminal.Term, string, bool) (string, error) {
		return passphrase, nil
	}
	return s
}

// Cipher returns the appropriate Cipher implementation based on the encryption
// type.
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

	if !s.validation {
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

	p, err := s.prompt(ctx, s.term, repo, s.confirmation)
	if err != nil {
		return "", err // nothing is stored, so the next call asks again
	}
	s.shared = p

	return p, nil
}

// defaultPrompt asks passphrase for repos that are about to be encrypted.
func defaultPrompt(w io.Writer) PromptFunc {
	return func(ctx context.Context, t *terminal.Term, repo string, confirm bool) (string, error) {
		fmt.Fprintf(w, "Password for %q: ", repo)
		p1, err := t.InputPassword(ctx)
		if err != nil {
			return "", err
		}

		defer t.ReplaceLine("")

		if !confirm {
			return p1, nil
		}

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

func validatePassphrase(ctx context.Context, root, passphrase string) (string, error) {
	path, err := findSample(ctx, root, locker.Extension.String())
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
func findSample(ctx context.Context, dir, suffix string) (string, error) {
	var found string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, suffix) {
			found = path
			return fs.SkipAll
		}

		return nil
	})

	return found, err
}
