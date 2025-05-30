// Package qr provides utilities for generate, render and working with QR-Codes
package qr

import (
	"errors"
	"fmt"
	"os"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/mateconpizza/gm/internal/sys"
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
	var err error

	q.QR, err = qrcode.New(q.From, qrcode.High)
	if err != nil {
		return fmt.Errorf("generating qr-code: %w", err)
	}

	return nil
}

// GenerateImg generates the PNG from the QR-Code.
func (q *QRCode) GenerateImg(s string) error {
	if q.QR == nil {
		return ErrQRNotGenerated
	}

	var err error
	q.file, err = generatePNG(q.QR, s)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	return nil
}

// Open opens a QR-Code image in the system default image viewer.
func (q *QRCode) Open() error {
	if q.file == nil {
		return ErrQRFileNotFound
	}

	args := make([]string, 0)

	// FIX: remove display, keep default system
	if sys.BinExists("display") {
		args = append(args, "display", q.file.Name())
	} else {
		args = append(sys.OSArgs(), q.file.Name())
	}

	if err := sys.ExecuteCmd(args...); err != nil {
		return fmt.Errorf("%w: opening QR", err)
	}

	return nil
}

// Label adds a label to an image, with the given position (top or bottom).
func (q *QRCode) Label(s, pos string) error {
	if q.file == nil {
		return ErrQRFileNotFound
	}

	return addLabel(q.file.Name(), s, pos)
}

// Render renders a QR-Code to the standard output.
func (q *QRCode) Render() {
	fmt.Print(q.QR.ToSmallString(true))
}

func (q *QRCode) String() string {
	return q.QR.ToSmallString(true)
}

// New creates a new QR-Code.
func New(s string) *QRCode {
	return &QRCode{
		From: s,
	}
}
