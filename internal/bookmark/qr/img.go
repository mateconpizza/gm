package qr

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"

	"github.com/haaag/gm/internal/sys/files"
)

// position is a position on an image.
type position struct {
	x, y int
}

// RenderOpts contains options for rendering, including image, font, and
// position calculation function.
type RenderOpts struct {
	bitmap  *image.RGBA
	face    *basicfont.Face
	calcPos func(string, *font.Drawer) position
}

// loadImage opens an image file and decodes it as an `image.Image`.
func loadImage(s string) (image.Image, error) {
	f, err := os.Open(s)
	if err != nil {
		return nil, fmt.Errorf("opening image: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("error closing source file: %v", err)
		}
	}()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	return img, nil
}

// createFontDrawer creates a font drawer with the given label.
func createFontDrawer(s string, ro RenderOpts) *font.Drawer {
	fd := &font.Drawer{
		Dst:  ro.bitmap,
		Src:  image.NewUniform(color.RGBA{0, 0, 0, 255}), // black
		Face: ro.face,
	}

	pos := ro.calcPos(s, fd)

	// Set the position for the drawer
	fd.Dot = fixed.Point26_6{X: fixed.I(pos.x), Y: fixed.I(pos.y)}

	return fd
}

// addLabel adds a label to an image, with the given position.
func addLabel(path, text, pos string) error {
	img, err := loadImage(path)
	if err != nil {
		return err
	}

	// Convert the image to RGBA
	bitmap := image.NewRGBA(img.Bounds())
	draw.Draw(bitmap, bitmap.Bounds(), img, image.Point{}, draw.Src)

	opts := RenderOpts{
		bitmap: bitmap,
	}

	switch pos {
	case "top":
		opts.face = inconsolata.Bold8x16
		opts.calcPos = calcTop
	default:
		opts.face = inconsolata.Regular8x16
		opts.calcPos = calcBottom
	}

	fd := createFontDrawer(text, opts)

	// Draw the label
	fd.DrawString(text)

	// Save the image with the label
	f, err := files.Touch(path, true)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("error closing source file: %v", err)
		}
	}()

	// Write image
	err = png.Encode(f, bitmap)
	if err != nil {
		return fmt.Errorf("encoding image: %w", err)
	}

	return nil
}

// calcBottom calculates the position for the bottom label.
func calcBottom(s string, fd *font.Drawer) position {
	// Measure the label size
	labelWidth := fd.MeasureString(s).Ceil()
	labelHeight := fd.Face.Metrics().Height.Ceil()

	// Calculate the position to center the label
	x := (fd.Dst.Bounds().Dx() - labelWidth) / 2
	y := fd.Dst.Bounds().Dy() - labelHeight

	return position{x, y}
}

// calcTop calculates the position for the top label.
func calcTop(s string, fd *font.Drawer) position {
	// Measure the label size
	labelWidth := fd.MeasureString(s).Ceil()
	labelHeight := fd.Face.Metrics().Height.Ceil()

	// Calculate the position to center the label
	x := (fd.Dst.Bounds().Dx() - labelWidth) / 2
	y := labelHeight // Position from the top edge

	// Add one line of text height to y to move it one line down
	y += fd.Face.Metrics().Height.Ceil()

	return position{x, y}
}

// generatePNG generates a PNG from a given QR-Code.
func generatePNG(qr *qrcode.QRCode, prefix string) (*os.File, error) {
	const imgSize = 512
	qrfile, err := files.CreateTemp(prefix, "png")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	if err := qr.WriteFile(imgSize, qrfile.Name()); err != nil {
		return nil, fmt.Errorf("writing qr-code: %w", err)
	}

	return qrfile, nil
}
