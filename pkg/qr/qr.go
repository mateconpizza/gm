package qr

import (
	"fmt"
	"log"
	"os"

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
	qrfile, err := util.CreateTempFile(prefix)
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	if err := qr.WriteFile(512, qrfile.Name()); err != nil {
		return nil, fmt.Errorf("writing qr-code: %w", err)
	}

	return qrfile, nil
}

// Open opens a QR-Code image in the system default image viewer.
func Open(qr *qrcode.QRCode, prefix string) error {
	qrfile, err := generatePNG(qr, prefix)
	if err != nil {
		return err
	}

	args := util.GetOSArgsCmd()
	args = append(args, qrfile.Name())
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

// Render renders a QR-Code to the standard output.
func Render(qr *qrcode.QRCode) error {
	if _, err := fmt.Println(qr.ToSmallString(false)); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
