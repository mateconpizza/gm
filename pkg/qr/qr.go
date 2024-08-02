package qr

import (
	"fmt"
	"os"
	"strings"

	"github.com/skip2/go-qrcode"

	"github.com/haaag/gm/pkg/util"
)

// Generate generates a QR-Code from a given URL.
func Generate(url string) (*qrcode.QRCode, error) {
	qr, err := qrcode.New(url, qrcode.High)
	if err != nil {
		return nil, fmt.Errorf("generating qr-code: %w", err)
	}

	return qr, nil
}

// generatePNG generates a PNG from a given QR-Code.
func generatePNG(qr *qrcode.QRCode, prefix string) (*os.File, error) {
	const imgSize = 512

	qrfile, err := util.CreateTempFile(prefix, "png")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}

	if err := qr.WriteFile(imgSize, qrfile.Name()); err != nil {
		return nil, fmt.Errorf("writing qr-code: %w", err)
	}

	return qrfile, nil
}

// Render renders a QR-Code to the standard output.
func Render(qr *qrcode.QRCode, title, url string) {
	var sb strings.Builder
	sb.WriteString(title + "\n")
	sb.WriteString(qr.ToSmallString(false))
	sb.WriteString(url + "\n")
	fmt.Print(sb.String())
}
