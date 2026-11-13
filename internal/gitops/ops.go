package gitops

import (
	"context"
	"log/slog"
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

func NewRepo(gm *git.Mgr, name string, sess *session.Session, color bool, opts ...RepoOpsOptFunc) *git.Repo {
	opts = append([]RepoOpsOptFunc{WithSpinner(NewSpinner(color))}, opts...)
	f := NewRepoOps(sess, opts...)

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
		logger  *slog.Logger
	}

	RepoOpsOptFunc func(*RepoOps)
)

func NewRepoOps(sess *session.Session, opts ...RepoOpsOptFunc) *RepoOps {
	ro := &RepoOps{
		session: sess,
		spinner: rotato.New(),
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(ro)
	}
	ro.logger = ro.logger.With("component", "repo-ops")

	return ro
}

func WithSpinner(sp spinner) RepoOpsOptFunc {
	return func(ro *RepoOps) {
		if sp != nil {
			ro.spinner = sp
		}
	}
}

func WithRepoOpsLogger(l *slog.Logger) RepoOpsOptFunc {
	return func(ro *RepoOps) {
		if l != nil {
			ro.logger = l
		}
	}
}

func (ro *RepoOps) Reader() git.RepoOptFunc  { return git.WithRepoReader(ro.readFiles) }
func (ro *RepoOps) Writer() git.RepoOptFunc  { return git.WithRepoWriter(ro.writeFiles) }
func (ro *RepoOps) Remover() git.RepoOptFunc { return git.WithRepoRemover(ro.removeFiles) }

func (ro *RepoOps) readFiles(ctx context.Context, path string, total int) ([]*bookmark.Bookmark, error) {
	log := ro.logger.With("fn", "read-files", "repo", filepath.Base(path))

	root := filepath.Dir(path)
	rc, err := resolveConfig(root)
	if err != nil {
		return nil, err
	}
	log.Debug("reading repo", "path", path, "total", total, "format", rc.Format, "encryption", rc.Encryption)

	serializer, err := serializerFor(rc.Format)
	if err != nil {
		return nil, err
	}

	cipher, err := ro.session.NewCipher(ctx, root, filepath.Base(path), rc.Encryption)
	if err != nil {
		return nil, err
	}

	bs, err := newFileRepoReader(ctx, &readerCfg{
		name:       filepath.Base(path),
		root:       filepath.Dir(path),
		repoPath:   path,
		total:      total,
		spinner:    ro.spinner,
		serializer: serializer,
		cipher:     cipher,
		logger:     log,
	})
	if err != nil {
		return nil, err
	}

	log.Debug("repo read", "count", len(bs))

	return bs, nil
}

func (ro *RepoOps) writeFiles(ctx context.Context, path string, bs []*bookmark.Bookmark) error {
	log := ro.logger.With("fn", "write-files", "repo", filepath.Base(path))

	root := filepath.Dir(path)
	rc, err := resolveConfig(root)
	if err != nil {
		return err
	}
	log.Debug("writing repo", "path", path, "count", len(bs), "format", rc.Format, "encryption", rc.Encryption)

	serializer, err := serializerFor(rc.Format)
	if err != nil {
		return err
	}

	cipher, err := ro.session.NewCipher(ctx, root, filepath.Base(path), rc.Encryption)
	if err != nil {
		return err
	}

	err = newFileRepoWriter(ctx, &writerCfg{
		name:       filepath.Base(path),
		root:       root,
		repoPath:   path,
		bookmarks:  bs,
		spinner:    ro.spinner,
		cipher:     cipher,
		serializer: serializer,
	})
	if err != nil {
		return err
	}

	log.Debug("repo written", "count", len(bs))

	return nil
}

func (ro *RepoOps) removeFiles(ctx context.Context, repoPath string, bs []*bookmark.Bookmark) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	log := ro.logger.With("fn", "remove-files", "repo", filepath.Base(repoPath))

	root := filepath.Dir(repoPath)
	rc, err := resolveConfig(root)
	if err != nil {
		return err
	}
	log.Debug("removing files", "path", repoPath, "count", len(bs), "format", rc.Format)

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
		log.Debug("resolved file to remove", "id", b.ID, "file", filename)

		return fullpath, nil
	})
	if err != nil {
		return err
	}

	if err := r.Rm(ctx, bs); err != nil {
		return err
	}

	log.Debug("files removed", "count", len(bs))

	return nil
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
