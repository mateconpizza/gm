package gpg

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/pkg/files"
)

var testData = []byte(`tru::1:1712345678:0:3:1:5
pub:u:4096:1:A1B2C3D4E5F6G7H8:2022-05-20:2032-05-18::u:Pepe Hongo <pepe.hongo@example.com>::scESC:
fpr:::::::::A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6Q7R8S9T0:
uid:u::::1712345678::X1Y2Z3A4B5C6D7E8F9G0H1I2J3K4L5::Pepe Hongo (Personal) <pepe@hongo-family.org>:
uid:u::::1712345678::M1N2O3P4Q5R6S7T8U9V0W1X2Y3Z4A5::Pepe Hongo (Work) <phongo@company.com>:
sub:u:4096:1:S1T2U3V4W5X6Y7Z8:2022-05-20:2032-05-18::e:
fpr:::::::::S1T2U3V4W5X6Y7Z8A9B0C1D2E3F4G5H6I7J8K9L0:
pub:u:2048:1:C9D8E7F6A5B4C3D2:2023-11-10:2025-11-08::u:Palan Palandri <palan.palandri@example.org>::scaSC:
fpr:::::::::C9D8E7F6A5B4C3D2E1F0A9B8C7D6E5F4A3B2C1D0E9F8A7B6:
uid:u::::1712345678::Z9Y8X7W6V5U4T3S2R1Q0P9O8N7M6L5::Palan Palandri <ppalandri@university.edu>:
uid:u::::1712345678::K9J8H7G6F5E4D3C2B1A0Z9Y8X7W6V5U4::Palan Palandri (Developer) <palan@github.com>:
uid:u::::1712345678::R9S8T7U6V5W4X3Y2Z1A0B9C8D7E6F5G4::Dr. Palan Palandri <p.palandri@research-institute.org>:
sub:u:2048:1:E1F2G3H4I5J6K7L8:2023-11-10:2025-11-08::s:
fpr:::::::::E1F2G3H4I5J6K7L8M9N0O1P2Q3R4S5T6U7V8W9X0:
sub:u:2048:1:M8N7O6P5Q4R3S2T1:2023-11-10:2024-11-09::e:
fpr:::::::::M8N7O6P5Q4R3S2T1U0V9W8X7Y6Z5A4B3C2D1E0F9:
pub:u:3072:1:B3C4D5E6F7A8B9C0:2024-01-15:2026-01-14::u:Pepe Hongo & Palan Palandri <team@collab-project.com>::scESC:
fpr:::::::::B3C4D5E6F7A8B9C0D1E2F3A4B5C6D7E8F9A0B1C2D3E4F5:
uid:u::::1712345678::D5E6F7A8B9C0D1E2F3A4B5C6D7E8F9A0B1::Joint Project Key:::
sub:u:3072:1:N2O3P4Q5R6S7T8U9:2024-01-15:2026-01-14::e:
fpr:::::::::N2O3P4Q5R6S7T8U9V0W1X2Y3Z4A5B6C7D8E9F0G1:`)

func TestParseGPGOutput(t *testing.T) {
	t.Parallel()
	keys, err := parseGPGOutput(testData)
	if err != nil {
		t.Fatalf("parseGPGOutput returned error: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("expected some keys, got none")
	}
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	first := keys[0]
	if first.KeyID != "A1B2C3D4E5F6G7H8" {
		t.Errorf("unexpected KeyID: %s", first.KeyID)
	}
	if !strings.Contains(first.UserID, "Pepe Hongo") {
		t.Errorf("unexpected UserID: %s", first.UserID)
	}
	if first.Fingerprint == "" {
		t.Error("expected fingerprint, got empty")
	}
	if !first.IsPrimaryKey {
		t.Error("expected IsPrimaryKey to be true")
	}
	if len(first.Subkeys) == 0 {
		t.Error("expected at least one subkey")
	}
}

// TestParseGPGOutputEmpty tests parsing with empty output.
func TestParseGPGOutputEmpty(t *testing.T) {
	t.Parallel()
	keys, err := parseGPGOutput([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// TestFingerprintTrust tests trust level helpers.
func TestFingerprintTrust(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		trustLevel string
		wantTrust  bool
		wantString string
	}{
		{"ultimate", "u", true, "ultimate"},
		{"full", "f", true, "full"},
		{"marginal", "m", false, "marginal"},
		{"never", "n", false, "never"},
		{"unknown", "o", false, "unknown"},
		{"empty", "", false, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fp := &Fingerprint{TrustLevel: tt.trustLevel}

			if got := fp.IsTrusted(); got != tt.wantTrust {
				t.Errorf("IsTrusted() = %v, want %v", got, tt.wantTrust)
			}

			if got := fp.TrustLevelString(); got != tt.wantString {
				t.Errorf("TrustLevelString() = %v, want %v", got, tt.wantString)
			}
		})
	}
}

// TestLoadFingerprint tests loading fingerprint from file.
func TestLoadFingerprint(t *testing.T) {
	t.Parallel()
	// Create temp directory
	tmpDir := t.TempDir()
	gpgIDPath := filepath.Join(tmpDir, fingerprintIDFilename)

	// Test: file doesn't exist
	_, err := loadFingerprint(gpgIDPath)
	if err == nil {
		t.Error("expected error when file doesn't exist")
	}

	// Test: empty file
	if err := os.WriteFile(gpgIDPath, []byte(""), files.FilePerm); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	_, err = loadFingerprint(gpgIDPath)
	if !errors.Is(err, ErrNoFingerprint) {
		t.Errorf("expected ErrNoFingerprint, got %v", err)
	}

	// Test: valid fingerprint
	testFingerprint := "A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6Q7R8S9T0"
	if err := os.WriteFile(gpgIDPath, []byte(testFingerprint+"\n"), files.FilePerm); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	fp, err := loadFingerprint(gpgIDPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp != testFingerprint {
		t.Errorf("expected %s, got %s", testFingerprint, fp)
	}

	// Test: fingerprint with whitespace
	if err := os.WriteFile(gpgIDPath, []byte("  "+testFingerprint+"  \n"), files.FilePerm); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	fp, err = loadFingerprint(gpgIDPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp != testFingerprint {
		t.Errorf("expected %s, got %s", testFingerprint, fp)
	}
}

// TestIsInitialized tests the IsInitialized function.
func TestIsInitialized(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Test: not initialized
	if IsInitialized(tmpDir) {
		t.Error("expected false for uninitialized directory")
	}

	// Test: initialized
	gpgIDPath := filepath.Join(tmpDir, fingerprintIDFilename)
	if err := os.WriteFile(gpgIDPath, []byte("A1B2C3D4E5F6G7H8\n"), files.FilePerm); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if !IsInitialized(tmpDir) {
		t.Error("expected true for initialized directory")
	}
}

// TestFingerprintIDPath tests the path helper.
func TestFingerprintIDPath(t *testing.T) {
	t.Parallel()
	path := "/test/path"
	expected := filepath.Join(path, fingerprintIDFilename)
	if got := GPGIDPath(path); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

// TestGPGEncryptNoRecipient tests encryption without recipient.
func TestGPGEncryptNoRecipient(t *testing.T) {
	t.Parallel()
	g := &GPG{
		Recipient: "",
		BinPath:   "/usr/bin/gpg",
	}

	err := g.Encrypt(t.Context(), "test.gpg", []byte("data"))
	if !errors.Is(err, ErrNoGPGRecipient) {
		t.Errorf("expected ErrNoGPGRecipient, got %v", err)
	}
}

// TestInit tests the Init function.
func TestInit(t *testing.T) {
	t.Parallel()
	_, err := sys.Which(Command)
	if err != nil {
		// This test requires gpg to be installed
		// Skip if gpg is not available
		t.Skip("skipping integration test - requires gpg binary")
	}

	tmpDir := t.TempDir()
	fp := &Fingerprint{
		Fingerprint: "A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6Q7R8S9T0",
	}

	err = Init(tmpDir, ".gitattributes", fp)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify .gpg-id was created
	gpgIDPath := filepath.Join(tmpDir, fingerprintIDFilename)
	content, err := os.ReadFile(gpgIDPath)
	if err != nil {
		t.Fatalf("failed to read .gpg-id: %v", err)
	}
	expected := fp.Fingerprint + "\n"
	if string(content) != expected {
		t.Errorf("expected %q, got %q", expected, string(content))
	}

	// Verify .gitattributes was created
	gitAttrPath := filepath.Join(tmpDir, ".gitattributes")
	content, err = os.ReadFile(gitAttrPath)
	if err != nil {
		t.Fatalf("failed to read .gitattributes: %v", err)
	}
	if string(content) != gitAttContent {
		t.Errorf("expected %q, got %q", gitAttContent, string(content))
	}
}

// mockExecSuccess returns a Cmd that prints fake output.
func mockExecSuccess(output string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", output)
	}
}

// mockExecFail returns a Cmd that exits with error.
func mockExecFail(stderr string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "echo '"+stderr+"' 1>&2; exit 1")
	}
}

func TestGPG_Encrypt_Success(t *testing.T) {
	t.Parallel()
	execCommand = mockExecSuccess("encrypted ok")

	g := &GPG{Recipient: "user@example.com", BinPath: "/usr/bin/gpg"}
	err := g.Encrypt(t.Context(), "output.gpg", []byte("hello"))
	if err != nil {
		t.Fatalf("Encrypt failed unexpectedly: %v", err)
	}
}

func TestGPG_Encrypt_NoRecipient(t *testing.T) {
	t.Parallel()
	g := &GPG{}
	err := g.Encrypt(t.Context(), "file", []byte("data"))
	if !errors.Is(err, ErrNoGPGRecipient) {
		t.Fatalf("expected ErrNoGPGRecipient, got %v", err)
	}
}

func TestGPG_Encrypt_CommandFails(t *testing.T) {
	t.Parallel()
	execCommand = mockExecFail("some gpg error")

	g := &GPG{Recipient: "user@example.com", BinPath: "/usr/bin/gpg"}

	err := g.Encrypt(t.Context(), "output.gpg", []byte("hello"))
	if err == nil {
		t.Fatal("expected error from Encrypt, got nil")
	}
	if !strings.Contains(err.Error(), "gpg encrypt failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGPG_Decrypt_Success(t *testing.T) {
	t.Parallel()
	execCommand = mockExecSuccess("decrypted text")

	g := &GPG{BinPath: "/usr/bin/gpg"}
	out, err := g.Decrypt(t.Context(), "file.gpg")
	if err != nil {
		t.Fatalf("Decrypt failed unexpectedly: %v", err)
	}
	if string(bytes.TrimSpace(out)) != "decrypted text" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestGPG_Decrypt_CommandFails(t *testing.T) {
	t.Parallel()
	execCommand = mockExecFail("bad decrypt")

	g := &GPG{BinPath: "/usr/bin/gpg"}
	_, err := g.Decrypt(t.Context(), "file.gpg")
	if err == nil {
		t.Fatal("expected error from Decrypt, got nil")
	}
	if !strings.Contains(err.Error(), "gpg decrypt failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// Benchmark tests.
func BenchmarkParseGPGOutput(b *testing.B) {
	for b.Loop() {
		_, _ = parseGPGOutput(testData)
	}
}
