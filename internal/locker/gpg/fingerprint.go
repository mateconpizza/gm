package gpg

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// Fingerprint represents a GPG key and its subkeys.
type Fingerprint struct {
	KeyID        string
	UserID       string
	Fingerprint  string   // main key fingerprint
	Subkeys      []string // list of subkey fingerprints
	SubkeyIDs    []string // list of subkey short IDs
	IsPrimaryKey bool
	TrustLevel   string
}

// IsTrusted returns true if the key is fully or ultimately trusted.
func (f *Fingerprint) IsTrusted() bool {
	return f.TrustLevel == "f" || f.TrustLevel == "u"
}

// TrustLevelString returns a human-readable trust level.
func (f *Fingerprint) TrustLevelString() string {
	switch f.TrustLevel {
	case "u":
		return "ultimate"
	case "f":
		return "full"
	case "m":
		return "marginal"
	case "n":
		return "never"
	case "q", "o", "-", "":
		return "unknown"
	default:
		return "unknown"
	}
}

func (f *Fingerprint) String() string {
	return fmt.Sprintf("ID: %s  User: %s\nFingerprint: %s", f.KeyID, f.UserID, f.Fingerprint)
}

// ListFingerprints lists all public GPG keys with their fingerprints and subkeys.
func ListFingerprints() ([]*Fingerprint, error) {
	output, err := execGPGListKeys()
	if err != nil {
		return nil, err
	}

	fps, err := parseGPGOutput(output)
	if err != nil {
		return nil, err
	}

	if len(fps) == 0 {
		return nil, ErrNoFingerprint
	}

	return fps, nil
}

// loadFingerprint loads fingerprint from the .gpg-id file.
func loadFingerprint(f string) (string, error) {
	slog.Debug("gpg: loading GPG fingerprint", "path", f)

	if !fileExists(f) {
		slog.Error("gpg: gpg-id file does not exist", "path", f)
		return "", fmt.Errorf("%w: %q", ErrNoGPGIDFile, f)
	}

	fingerprint, err := os.ReadFile(f)
	if err != nil {
		slog.Error("gpg: failed to read gpg-id file", "path", f, "error", err)
		return "", fmt.Errorf("failed to read .gpg-id: %w", err)
	}

	recipientKey := strings.TrimSpace(string(fingerprint))
	if recipientKey == "" {
		slog.Error("gpg: empty fingerprint in gpg-id file", "path", f)
		return "", ErrNoFingerprint
	}

	slog.Debug(
		"gpg: loaded GPG fingerprint successfully",
		"path", f,
		"fingerprint", recipientKey,
	)

	return recipientKey, nil
}

// execGPGListKeys executes the GPG command and returns its raw colon-delimited output.
func execGPGListKeys() ([]byte, error) {
	binPath, err := which()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, Command)
	}

	cmd := exec.Command(
		binPath,
		flags.listKeys,
		flags.withColons,
		flags.fingerprint,
		flags.batch,
		flags.quiet,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gpg list-keys failed to execute: %w", err)
	}

	return output, nil
}

// parseGPGOutput processes the raw colon-delimited GPG output into Fingerprint structs.
func parseGPGOutput(output []byte) ([]*Fingerprint, error) {
	const (
		trustFieldIndex = 1
		keyIDFieldIndex = 4
		fprFieldIndex   = 9
		uidFieldIndex   = 9
	)

	var (
		keys    []*Fingerprint
		current *Fingerprint
		lastTag string
	)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) <= fprFieldIndex {
			continue
		}
		tag := fields[0]

		switch tag {
		case "pub":
			current = handlePub(fields, trustFieldIndex, keyIDFieldIndex)
			keys = append(keys, current)
		case "uid":
			handleUID(current, fields, uidFieldIndex)
		case "fpr":
			handleFPR(current, fields, fprFieldIndex, lastTag)
		case "sub":
			handleSub(current, fields, keyIDFieldIndex)
		}
		lastTag = tag
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error during parsing: %w", err)
	}
	return keys, nil
}

func handlePub(fields []string, trustIdx, keyIdx int) *Fingerprint {
	return &Fingerprint{
		KeyID:        fields[keyIdx],
		TrustLevel:   fields[trustIdx],
		IsPrimaryKey: true,
	}
}

func handleUID(current *Fingerprint, fields []string, uidIdx int) {
	if current != nil && current.UserID == "" {
		current.UserID = strings.TrimSpace(fields[uidIdx])
	}
}

func handleFPR(current *Fingerprint, fields []string, fprIdx int, lastTag string) {
	if current == nil {
		return
	}
	fp := strings.TrimSpace(fields[fprIdx])
	switch lastTag {
	case "pub":
		current.Fingerprint = fp
	case "sub":
		current.Subkeys = append(current.Subkeys, fp)
	}
}

func handleSub(current *Fingerprint, fields []string, keyIdx int) {
	if current != nil {
		current.SubkeyIDs = append(current.SubkeyIDs, fields[keyIdx])
	}
}
