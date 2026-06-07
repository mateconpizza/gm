package gpg

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	ErrKeyExpired    = errors.New("gpg: key has expired")
	ErrKeyNotFound   = errors.New("gpg: key not found")
	ErrKeyNotTrusted = errors.New("gpg: key not trusted")
	ErrKeyUnusable   = errors.New("gpg: unusable public key")
)

type TrustLevel string

const (
	TrustUltimate TrustLevel = "u"
	TrustFull     TrustLevel = "f"
	TrustMarginal TrustLevel = "m"
	TrustExpired  TrustLevel = "e"
	TrustNever    TrustLevel = "n"
	TrustUnknown  TrustLevel = "q" // also “o”, “-”, or empty string.
)

// Fingerprint represents a GPG key and its subkeys.
type Fingerprint struct {
	KeyID        string
	UserID       string
	Fingerprint  string   // main key fingerprint
	Subkeys      []string // list of subkey fingerprints
	SubkeyIDs    []string // list of subkey short IDs
	IsPrimaryKey bool
	TrustLevel   TrustLevel
}

// IsTrusted returns true if the key is fully or ultimately trusted.
func (f *Fingerprint) IsTrusted() bool {
	return f.TrustLevel == TrustFull || f.TrustLevel == TrustUltimate
}

func (f *Fingerprint) Expired() bool {
	return f.TrustLevel == TrustExpired
}

// TrustLevelString returns a human-readable trust level.
func (f *Fingerprint) TrustLevelString() string {
	switch f.TrustLevel {
	case TrustUltimate:
		return "ultimate"
	case TrustFull:
		return "full"
	case TrustExpired:
		return "expired"
	case TrustMarginal:
		return "marginal"
	case TrustNever:
		return "never"
	case TrustUnknown, "o", "-", "":
		return "unknown"
	default:
		return "unknown"
	}
}

func (f *Fingerprint) Validate() error {
	if f.Expired() {
		return fmt.Errorf("%w: %s %q", ErrKeyExpired, f.UserID, f.Fingerprint)
	}
	if !f.IsTrusted() {
		return fmt.Errorf("%w: %s %q", ErrKeyNotTrusted, f.UserID, f.Fingerprint)
	}
	return nil
}

func (f *Fingerprint) String() string {
	return fmt.Sprintf("ID: %s  User: %s\nFingerprint: %s", f.KeyID, f.UserID, f.Fingerprint)
}

// LookupKey looks up the GPG key for the fingerprint stored in path.
func LookupKey(path string) (*Fingerprint, error) {
	if !fileExists(path) {
		return nil, fmt.Errorf("%w: %q", os.ErrNotExist, path)
	}

	recipient, err := loadFingerprint(path)
	if err != nil {
		return nil, err
	}

	if recipient == "" {
		return nil, ErrNoGPGRecipient
	}

	fps, err := ListFingerprints()
	if err != nil {
		return nil, err
	}

	if len(fps) == 0 {
		return nil, ErrNoFingerprint
	}

	for i := range fps {
		if fps[i].Fingerprint == recipient {
			return fps[i], nil
		}
	}

	return nil, ErrNoFingerprint
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
	if !fileExists(f) {
		return "", fmt.Errorf("%w: %q", ErrNoGPGIDFile, f)
	}

	fingerprint, err := os.ReadFile(f)
	if err != nil {
		return "", fmt.Errorf("failed to read .gpg-id: %w", err)
	}

	recipientKey := strings.TrimSpace(string(fingerprint))
	if recipientKey == "" {
		return "", ErrNoFingerprint
	}

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
		TrustLevel:   TrustLevel(fields[trustIdx]),
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
