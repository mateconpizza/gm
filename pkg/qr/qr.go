package qr

import (
	"errors"
	"fmt"
	"os"

	"github.com/skip2/go-qrcode"

	"github.com/haaag/gm/pkg/util"
	"github.com/haaag/gm/pkg/util/files"
)

var (
	ErrQRFileNotFound = errors.New("QR-Code file not found")
	ErrQRNotGenerated = errors.New("QR-Code not generated")
)

// QRCode represents a QR-Code.
type QRCode struct {
	QR   *qrcode.QRCode
	file *os.File
	From string
}

// Generate generates a QR-Code from a given string.
func (q *QRCode) Generate() error {
	qr, err := qrcode.New(q.From, qrcode.High)
	if err != nil {
		return fmt.Errorf("generating qr-code: %w", err)
	}

	q.QR = qr

	return nil
}

// GenImg generates the PNG from the QR-Code.
func (q *QRCode) GenImg(filename string) error {
	if q.QR == nil {
		return ErrQRNotGenerated
	}

	qrfile, err := generatePNG(q.QR, filename)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	q.file = qrfile

	return nil
}

// Open opens a QR-Code image in the system default image
// viewer.
func (q *QRCode) Open() error {
	if q.file == nil {
		return ErrQRFileNotFound
	}

	args := append(util.GetOSArgsCmd(), q.file.Name())
	if err := util.ExecuteCmd(args...); err != nil {
		return fmt.Errorf("%w: opening QR", err)
	}

	defer func() {
		if err := util.CleanupTempFile(q.file.Name()); err != nil {
			log.Printf("error cleaning up temp file %v", err)
		}
	}()

	return nil
}

// Label adds a label to an image, with the given position
// (top or bottom).
func (q *QRCode) Label(s, position string) error {
	if q.file == nil {
		return ErrQRFileNotFound
	}

	return addLabel(q.file.Name(), s, position)
}

// Render renders a QR-Code to the standard output.
func (q *QRCode) Render() {
	fmt.Print(q.QR.ToSmallString(false))
}

// New creates a new QR-Code.
func New(s string) *QRCode {
	return &QRCode{
		From: s,
	}
}
