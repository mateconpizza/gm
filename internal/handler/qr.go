package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrInvalidFormat = errors.New("invalid format")

// QRFormats image valid formats.
var QRFormats = []string{".jpeg", ".png", ".jpg"}

// QR handles creation, rendering or opening of QR-Codes.
func QR(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("fn", "qr")

	c, p := d.Console(), d.Console().Palette()
	n := len(bs)
	piped := c.IsPiped()
	log.Debug("showing qr-codes", "count", n, "piped", piped)

	for i := range bs {
		b := bs[i]
		blog := log.With("id", b.ID)

		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return err
		}
		blog.Debug("qr-code generated", "index", i+1, "total", n)

		var sb strings.Builder

		sb.WriteString(p.Bold.Sprint(b.Title))
		sb.WriteByte('\n')
		sb.WriteString(p.Italic.Sprint(b.URL))
		sb.WriteByte('\n')
		sb.WriteString(qrcode.String())

		output := sb.String()
		fmt.Fprint(d.Writer(), output)

		if piped {
			continue
		}

		if err := c.WaitForEnter(ctx, fmt.Sprintf("[%d/%d] Press ENTER to continue...", i+1, n)); err != nil {
			if errors.Is(err, context.Canceled) {
				blog.Debug("canceled by user while waiting", "index", i+1)
				return app.Abort()
			}
			return err
		}

		d.Console().Term().ClearLine(txt.CountLines(output))
	}

	log.Debug("qr-codes shown", "count", n)

	return nil
}

// QROpen opens a QR-Code image in the system default image viewer.
func QROpen(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("fn", "qr-open")

	var (
		count atomic.Uint32
		c, p  = d.Console(), d.Console().Palette()
		s     = fmt.Sprintf("%s %d bookmarks qr-code", p.BrightGreen.Wrap("open", p.Bold), len(bs))
	)

	log.Debug("opening qr-codes", "count", len(bs), "force", app.Flags.Force)

	if err := c.ConfirmLimit(ctx, len(bs), 10, s, app.Flags.Force); err != nil {
		return err
	}

	sp := NewSpinner(app.Flags.Color)
	sp.UpdateMesg("QR-Code")
	sp.AddMessageDecorator(func(prefix string) string {
		current := count.Load()
		return fmt.Sprintf("%s %s", p.Gray.Sprintf("[%d/%d]", current, len(bs)), prefix)
	})

	sp.Start(ctx)
	defer sp.Done()

	const maxLabelLen = 55

	for i := range bs {
		b := bs[i]
		blog := log.With("id", b.ID)

		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return err
		}

		if err := qrcode.GenerateImg(app.Name); err != nil {
			return err
		}

		trunc := func(s string) string { return txt.Shorten(s, maxLabelLen) }
		if err := qrcode.Label(trunc(b.Title), qr.LabelTop); err != nil {
			return fmt.Errorf("%w: adding top label", err)
		}

		if err := qrcode.Label(trunc(b.URL), qr.LabelBottom); err != nil {
			return fmt.Errorf("%w: adding bottom label", err)
		}

		count.Add(1)
		blog.Debug("opening image", "index", i+1, "total", len(bs))

		if err := qrcode.Open(ctx); err != nil {
			return err
		}
	}

	log.Debug("qr-codes opened", "count", len(bs))

	return nil
}

// QRSave writes a QR code image to a supported file format.
func QRSave(qrcode *qr.QRCode, path string) error {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".png", ".jpg", ".jpeg":
		fn, err := files.NormalizePath(path, "qr.png")
		if err != nil {
			return err
		}
		if err := qrcode.Write(fn); err != nil {
			return err
		}
		slog.Debug("qr-code saved", "fn", "qr-save", "path", fn)
		return nil

	default:
		return fmt.Errorf("%w: %q (use: %s)", ErrInvalidFormat, ext, strings.Join(QRFormats, ", "))
	}
}
