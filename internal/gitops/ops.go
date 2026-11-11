package gitops

import (
	"context"
	"path/filepath"
	"strings"

	files "github.com/mateconpizza/gofiles"
	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/session"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

func NewRepo(gm *git.Mgr, name string, sess *session.Session, color bool) *git.Repo {
	f := NewRepoOps(
		sess,
		WithSpinner(NewSpinner(color)),
	)

	return gm.NewRepo(name,
		f.Reader(),
		f.Writer(),
		f.Remover(),
	)
}

type (
	RepoOps struct {
		session *session.Session
		spinner spinner
	}

	RepoOpsOptFunc func(*RepoOps)
)

func NewRepoOps(sess *session.Session, opts ...RepoOpsOptFunc) *RepoOps {
	ro := &RepoOps{
		session: sess,
		spinner: rotato.New(),
	}
	for _, opt := range opts {
		opt(ro)
	}
	return ro
}

func WithSpinner(sp spinner) RepoOpsOptFunc {
	return func(ro *RepoOps) {
		if sp != nil {
			ro.spinner = sp
		}
	}
}

func (ro *RepoOps) Reader() git.RepoOptFunc  { return git.WithRepoReader(ro.readFiles) }
func (ro *RepoOps) Writer() git.RepoOptFunc  { return git.WithRepoWriter(ro.writeFiles) }
func (ro *RepoOps) Remover() git.RepoOptFunc { return git.WithRepoRemover(removeFiles) }

func (ro *RepoOps) readFiles(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error) {
	root := filepath.Dir(path)
	rc, err := resolveConfig(root)
	if err != nil {
		return nil, err
	}

	serializer, err := serializerFor(rc.Format)
	if err != nil {
		return nil, err
	}

	cipher, err := ro.session.NewCipher(ctx, root, filepath.Base(path), rc.Encryption)
	if err != nil {
		return nil, err
	}

	return newFileRepoReader(ctx, &readerCfg{
		name:       filepath.Base(path),
		root:       filepath.Dir(path),
		repoPath:   path,
		total:      total,
		spinner:    ro.spinner,
		serializer: serializer,
		cipher:     cipher,
	})
}

func (ro *RepoOps) writeFiles(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
	root := filepath.Dir(path)
	rc, err := resolveConfig(root)
	if err != nil {
		return err
	}

	serializer, err := serializerFor(rc.Format)
	if err != nil {
		return err
	}

	cipher, err := ro.session.NewCipher(ctx, root, filepath.Base(path), rc.Encryption)
	if err != nil {
		return err
	}

	return newFileRepoWriter(ctx, &writerCfg{
		name:       filepath.Base(path),
		root:       root,
		repoPath:   path,
		bookmarks:  bs,
		spinner:    ro.spinner,
		cipher:     cipher,
		serializer: serializer,
	})
}

func removeFiles(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	root := filepath.Dir(repoPath)
	rc, err := resolveConfig(root)
	if err != nil {
		return err
	}

	serializer, err := serializerFor(rc.Format)
	if err != nil {
		return err
	}

	sess := session.New().
		WithPassphrase("").
		WithValidation(false)

	cipher, err := sess.NewCipher(ctx, root, filepath.Base(repoPath), rc.Encryption)
	if err != nil {
		return err
	}

	r, err := bookio.NewFileRemover(repoPath, files.DefaultManager, func(b *bookmark.Bookmark) (string, error) {
		filename, err := cipher.Path(b, serializer.Ext())
		if err != nil {
			return "", err
		}

		if !strings.HasSuffix(filename, cipher.Suffix()) {
			filename += cipher.Suffix()
		}

		fullpath := filepath.Join(repoPath, filename)
		return fullpath, nil
	})
	if err != nil {
		return err
	}

	return r.Rm(ctx, bs)
}

func NewSpinner(color bool) *rotato.Rotato {
	p := ansi.NewPalette(color)

	return rotato.New(
		rotato.WithMessage("starting..."),
		rotato.WithPrefixColor(p.Gray.Format),
		rotato.WithSpinnerColor(p.BrightYellow.With(p.Bold).Format),
		rotato.WithMessageColor(p.BrightBlue.With(p.Italic).Format),
		rotato.WithFailSymbolColor(p.BrightRed.With(p.Bold).Format),
		rotato.WithFailMessageColor(p.BrightRed.With(p.Bold).Format),
	)
}
