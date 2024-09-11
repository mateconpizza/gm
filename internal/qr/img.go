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

	"github.com/haaag/gm/internal/util/files"
)

// Pos is a position on an image.
type Pos struct {
	x, y int
}

// calcPosFn is a function that calculates a position on an image.
type calcPosFn func(rgba *image.RGBA, d *font.Drawer, s string, fontFace *basicfont.Face) Pos

// loadImage opens an image file and decodes it as an `image.Image`.
func loadImage(fileName string) (image.Image, error) {
	f, err := os.Open(fileName)
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
	calcPosition calcPosFn,
) *font.Drawer {
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.NewUniform(color.RGBA{0, 0, 0, 255}), // black
		Face: fontFace,
	}

	pos := calcPosition(rgba, d, label, fontFace)

	// Set the position for the drawer
	d.Dot = fixed.Point26_6{X: fixed.I(pos.x), Y: fixed.I(pos.y)}

	return d
}

// addLabel adds a label to an image, with the given position.
func addLabel(fileName, label, position string) error {
	img, err := loadImage(fileName)
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
	outFile, err := os.Create(fileName)
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
func calcBottomPos(rgba *image.RGBA, d *font.Drawer, s string, fontFace *basicfont.Face) Pos {
	// Measure the label size
	labelWidth := d.MeasureString(s).Ceil()
	labelHeight := fontFace.Metrics().Height.Ceil()

	// Calculate the position to center the label
	x := (rgba.Bounds().Dx() - labelWidth) / 2
	y := rgba.Bounds().Dy() - labelHeight

	return Pos{x, y}
}

// calcTopPos calculates the position for the top label.
func calcTopPos(rgba *image.RGBA, d *font.Drawer, s string, fontFace *basicfont.Face) Pos {
	// Measure the label size
	labelWidth := d.MeasureString(s).Ceil()
	labelHeight := fontFace.Metrics().Height.Ceil()

	// Calculate the position to center the label
	x := (rgba.Bounds().Dx() - labelWidth) / 2
	y := labelHeight // Position from the top edge

	// Add one line of text height to y to move it one line down
	y += fontFace.Metrics().Height.Ceil()

	return Pos{x, y}
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
