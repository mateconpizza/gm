package qr

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"

	"github.com/haaag/gm/pkg/util"
)

type Position struct {
	x, y int
}

type calculateFn func(rgba *image.RGBA, d *font.Drawer, label string) Position

// Open opens a QR-Code image in the system default image viewer.
func Open(qr *qrcode.QRCode, prefix, topLabel, bottomLabel string) error {
	qrfile, err := generatePNG(qr, prefix)
	if err != nil {
		return err
	}

	if err := addLabel(qrfile.Name(), topLabel, "top"); err != nil {
		return fmt.Errorf("%w: adding top label", err)
	}

	if err := addLabel(qrfile.Name(), bottomLabel, "bottom"); err != nil {
		return fmt.Errorf("%w: adding bottom label", err)
	}

	args := append(util.GetOSArgsCmd(), qrfile.Name())
	if err := util.ExecuteCmd(args...); err != nil {
		return fmt.Errorf("%w: opening QR", err)
	}

	defer func() {
		if err := util.CleanupTempFile(qrfile.Name()); err != nil {
			log.Printf("error cleaning up temp file %v", err)
		}
	}()

	return nil
}

// loadImage opens an image file and decodes it as an `image.Image`.
func loadImage(filename string) (image.Image, error) {
	f, err := os.Open(filename)
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
func createFontDrawer(
	rgba *image.RGBA,
	fontFace *basicfont.Face,
	label string,
	fn calculateFn,
) *font.Drawer {
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.NewUniform(color.RGBA{0, 0, 0, 255}), // black
		Face: fontFace,
	}

	pos := fn(rgba, d, label)

	// Set the position for the drawer
	d.Dot = fixed.Point26_6{X: fixed.I(pos.x), Y: fixed.I(pos.y)}

	return d
}

// addLabel adds a label to an image, with the given position.
func addLabel(filename, label, position string) error {
	img, err := loadImage(filename)
	if err != nil {
		return err
	}

	// Convert the image to RGBA
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	var d *font.Drawer
	switch position {
	case "top":
		d = createFontDrawer(rgba, inconsolata.Bold8x16, label, calcTopPos)
	case "bottom":
		d = createFontDrawer(rgba, inconsolata.Regular8x16, label, calcBottomPos)
	default:
		d = createFontDrawer(rgba, inconsolata.Regular8x16, label, calcBottomPos)
	}

	// Draw the label
	d.DrawString(label)

	// Save the image with the label
	outFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			log.Printf("error closing source file: %v", err)
		}
	}()

	// Write image
	err = png.Encode(outFile, rgba)
	if err != nil {
		return fmt.Errorf("encoding image: %w", err)
	}

	return nil
}

// calcBottomPos calculates the position for the bottom label.
func calcBottomPos(rgba *image.RGBA, d *font.Drawer, label string) Position {
	fontFace := inconsolata.Regular8x16

	// Measure the label size
	labelWidth := d.MeasureString(label).Ceil()
	labelHeight := fontFace.Metrics().Height.Ceil()

	// Calculate the position to center the label
	x := (rgba.Bounds().Dx() - labelWidth) / 2
	y := rgba.Bounds().Dy() - labelHeight

	return Position{x, y}
}

// calcTopPos calculates the position for the top label.
func calcTopPos(rgba *image.RGBA, d *font.Drawer, label string) Position {
	fontFace := inconsolata.Bold8x16

	// Measure the label size
	labelWidth := d.MeasureString(label).Ceil()
	labelHeight := fontFace.Metrics().Height.Ceil()

	// Calculate the position to center the label
	x := (rgba.Bounds().Dx() - labelWidth) / 2
	y := labelHeight // Position from the top edge

	// Add one line of text height to y to move it one line down
	y += fontFace.Metrics().Height.Ceil()

	return Position{x, y}
}
