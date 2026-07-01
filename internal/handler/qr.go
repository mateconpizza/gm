package handler

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

var ErrInvalidFormat = errors.New("invalid format")

// QRFormats image valid formats.
var QRFormats = []string{".jpeg", ".png", ".jpg"}

// QR handles creation, rendering or opening of QR-Codes.
func QR(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	c, p := d.Console(), d.Console().Palette()
	n := len(bs)

	for i := range bs {
		b := bs[i]
		qrcode := qr.New(b.URL)
		if err := qrcode.Generate(); err != nil {
			return err
		}

		var sb strings.Builder

		sb.WriteString(p.Bold.Sprint(b.Title))
		sb.WriteByte('\n')
		sb.WriteString(p.Italic.Sprint(b.URL))
		sb.WriteByte('\n')
		sb.WriteString(qrcode.String())

		output := sb.String()
		fmt.Fprint(d.Writer(), output)

		if c.IsPiped() {
			continue
		}

		if err := c.WaitForEnter(ctx, fmt.Sprintf("[%d/%d] Press ENTER to continue...", i+1, n)); err != nil {
			if errors.Is(err, context.Canceled) {
				return sys.ErrActionAborted
			}
			return err
		}

		terminal.ClearLine(txt.CountLines(output))
	}

	return nil
}

// QROpen opens a QR-Code image in the system default image viewer.
func QROpen(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	var (
		count atomic.Uint32
		c, p  = d.Console(), d.Console().Palette()
		dim   = rotato.FgGray.With(rotato.StyleBold)
		s     = fmt.Sprintf("%s %d bookmarks qr-code", p.BrightGreen.Wrap("open", p.Bold), len(bs))
	)

	if err := c.ConfirmLimit(ctx, len(bs), 10, s, app.Flags.Force); err != nil {
		return err
	}

	sp := rotato.New(
		rotato.WithSpinnerColor(rotato.FgBrightMagenta.With(rotato.StyleBold)),
		rotato.WithSpinnerStyle("block"),
		rotato.WithMessage("QR-Code"),
		rotato.WithMessageColor(rotato.FgBrightGreen),
		rotato.WithMessageDecorator(func(prefix string) string {
			current := count.Load()
			return fmt.Sprintf("%s %s", dim.Sprintf("[%d/%d]", current, len(bs)), prefix)
		}),
	)

	sp.Start(ctx)
	defer sp.Done()

	const maxLabelLen = 55

	for i := range bs {
		b := bs[i]
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
		if err := qrcode.Open(ctx); err != nil {
			return err
		}
	}

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
		return qrcode.Write(fn)

	default:
		return fmt.Errorf("%w: %q (use: %s)", ErrInvalidFormat, ext, strings.Join(QRFormats, ", "))
	}
}
