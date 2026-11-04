package enc

import (
	"context"
	"os"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

type Encryption string

const (
	None   Encryption = "none"
	GPG    Encryption = "gpg"
	AESGCM Encryption = "gcm"
)

type Cipher interface {
	Write(ctx context.Context, path string, content []byte) error
	Read(ctx context.Context, path string) ([]byte, error)
	Path(b *bookmark.Bookmark, ext string) (string, error)
	Suffix() string
}

var (
	_ Cipher = (*PlainCipher)(nil)
	_ Cipher = (*GPGCipher)(nil)
	_ Cipher = (*GCMCipher)(nil)
)

type PlainCipher struct{}

func (PlainCipher) Suffix() string                                        { return "" }
func (PlainCipher) Path(b *bookmark.Bookmark, ext string) (string, error) { return b.Path(ext) }
func (PlainCipher) Read(ctx context.Context, path string) ([]byte, error) { return os.ReadFile(path) }

func (PlainCipher) Write(ctx context.Context, path string, data []byte) error {
	return os.WriteFile(path, data, files.FilePerm)
}

type GCMCipher struct {
	passphrase string
}

func NewGCM(passphrase string) *GCMCipher {
	return &GCMCipher{passphrase: passphrase}
}

func (g *GCMCipher) Write(ctx context.Context, path string, content []byte) error {
	if err := os.WriteFile(path, content, files.FilePerm); err != nil {
		return err
	}
	return locker.Lock(ctx, path, g.passphrase)
}

func (g *GCMCipher) Read(ctx context.Context, path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return locker.Decrypt(ctx, b, g.passphrase)
}

func (g *GCMCipher) Path(b *bookmark.Bookmark, ext string) (string, error) {
	return b.PathEncrypted(ext)
}

func (g *GCMCipher) Suffix() string { return locker.Extension.String() }

type GPGCipher struct {
	encryptor func(ctx context.Context, path string, content []byte) error
	decryptor func(ctx context.Context, encryptedPath string) ([]byte, error)
}

func NewGPG(ctx context.Context, root string) (*GPGCipher, error) {
	k := gpg.NewKeyResolver(root)
	fp, err := k.Resolve(ctx)
	if err != nil {
		return nil, err
	}

	g := gpg.New(fp.Fingerprint)

	return &GPGCipher{
		encryptor: g.Encrypt,
		decryptor: g.Decrypt,
	}, nil
}

func (g *GPGCipher) Suffix() string { return gpg.Extension.String() }

func (g *GPGCipher) Write(ctx context.Context, path string, content []byte) error {
	return g.encryptor(ctx, path, content)
}

func (g *GPGCipher) Read(ctx context.Context, path string) ([]byte, error) {
	return g.decryptor(ctx, path)
}

func (g *GPGCipher) Path(b *bookmark.Bookmark, ext string) (string, error) {
	path, err := b.PathEncrypted(ext)
	if err != nil {
		return "", err
	}
	path = gpg.Extension.Join(path)
	return path, nil
}
