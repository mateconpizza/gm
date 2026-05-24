package gpg

// PinentryMode defines how GPG handles passphrase prompts.
type PinentryMode string

const (
	ModeDefault PinentryMode = "default" // Use the default of the agent, which is ask.
	ModeAsk     PinentryMode = "ask"     // Force the use of the Pinentry.
	ModeCancel  PinentryMode = "cancel"  // Emulate use of Pinentry's cancel button.
	ModeError   PinentryMode = "error"   // Return a Pinentry error (``No Pinentry'').

	// Redirect Pinentry queries to the caller.  Note that in contrast to
	// Pinentry the user is not prompted again if he enters a bad password.
	ModeLoopback PinentryMode = "loopback"
)

type gpgFlags struct {
	batch       string // Use batch mode.  Never ask, do not allow interactive commands.
	decrypt     string // Decrypt the file given on the command line
	encrypt     string // Encrypt data to one or more public keys.
	fingerprint string // List all keys (or the specified ones) along with their fingerprints.
	listKeys    string // List the specified keys.
	output      string // Write output to file.  To write to stdout use - as the filename.
	quiet       string // Try to be as quiet as possible.
	recipient   string // Encrypt for user id name.
	withColons  string // Print key listings delimited by colons.
	yes         string // Assume "yes" on most questions
}

// pinentryMode returns the dynamic CLI flag to set the pinentry mode.
func (gpgFlags) pinentryMode(pm PinentryMode) string {
	return "--pinentry-mode=" + string(pm)
}

// Arguments.
var flags = gpgFlags{
	batch:       "--batch",
	decrypt:     "--decrypt",
	encrypt:     "--encrypt",
	fingerprint: "--fingerprint",
	listKeys:    "--list-keys",
	output:      "--output",
	quiet:       "--quiet",
	recipient:   "--recipient",
	withColons:  "--with-colons",
	yes:         "--yes",
}

// GitDiffConf is the gpg diff configuration for git.
var GitDiffConf = map[string][]string{
	"diff.gpg.binary": {"true"},
	"diff.gpg.textconv": {
		Command,                // GPG executable.
		"-d",                   // Decrypt input.
		"--quiet",              // Try to be as quiet as possible.
		"--yes",                // Assume "yes" on most questions
		"--compress-algo=none", // Disable compression.
		"--no-encrypt-to",      // Ignore default encryption recipients.
		"--batch",              // Enable non-interactive mode.
		"--use-agent",          // Use the GPG agent.
	},
}
