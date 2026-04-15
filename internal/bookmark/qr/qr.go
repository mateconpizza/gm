// Package qr provides utilities for generate, render and working with QR-Codes
package qr

import (
	"context"
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

type labelPosition int

const (
	LabelTop labelPosition = iota
	LabelBottom
)

func (p labelPosition) String() string {
	switch p {
	case LabelTop:
		return "top"
	case LabelBottom:
		return "bottom"
	default:
		return ""
	}
}

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
func (q *QRCode) Open(ctx context.Context) error {
	if q.file == nil {
		return ErrQRFileNotFound
	}

	args := append(sys.OSArgs(), q.file.Name())
	if err := sys.ExecuteCmd(ctx, args...); err != nil {
		return fmt.Errorf("%w: opening QR", err)
	}

	return sys.ExecuteCmd(ctx, args...)
}

// Label adds a label to an image, with the given position (top or bottom).
func (q *QRCode) Label(s string, pos labelPosition) error {
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
