package gpg

import "fmt"

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
