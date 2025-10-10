package gpg

// Arguments.
var flags = struct {
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
}{
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
