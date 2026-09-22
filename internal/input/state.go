// Package input reads the state that gets sent to Jev
package input

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadState
// Finds the state text from the --state flag, then --state-file, then the reader which is usually stdin
// @param flagText {string} - the value of the --state flag
// @param filePath {string} - the value of the --state-file flag, - means the reader
// @param stdin {io.Reader} - the reader used when nothing else is given
// @return {string, error}
func ReadState(flagText, filePath string, stdin io.Reader) (string, error) {
	// Pick the first source that was given
	switch {
	case flagText != "":
		return flagText, nil
	case filePath == "-":
		return readAll(stdin)
	case filePath != "":
		// Read the file
		b, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read state file: %w", err)
		}

		// Return the trimmed contents
		return nonEmpty(string(b))
	default:
		return readAll(stdin)
	}
}

// readAll
// Reads the whole reader as the state, refusing a terminal so the command doesn't hang
// @param r {io.Reader} - the reader to drain
// @return {string, error}
func readAll(r io.Reader) (string, error) {
	// No reader or an interactive terminal means nothing was piped in
	if r == nil || isTerminal(r) {
		return "", errors.New("no state given: use --state, --state-file, or pipe stdin")
	}

	// Read everything
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}

	// Return the trimmed text
	return nonEmpty(string(b))
}

// nonEmpty
// Trims the text and errors when nothing is left
// @param s {string} - the text to check
// @return {string, error}
func nonEmpty(s string) (string, error) {
	// Trim the whitespace
	s = strings.TrimSpace(s)

	// Nothing left
	if s == "" {
		return "", errors.New("state is empty: use --state, --state-file, or pipe stdin")
	}

	// Return the trimmed text
	return s, nil
}

// ParseState
// Turns the state text into what goes on the wire, auto sends json objects and arrays as json and everything else as a string
// @param text {string} - the state text
// @param format {string} - auto, json or text
// @return {any, error}
func ParseState(text, format string) (any, error) {
	// Handle each format
	switch format {
	case "text":
		return text, nil
	case "json":
		// Try to decode it
		v, ok := tryJSON(text)

		// The caller said json but it isn't
		if !ok {
			return nil, errors.New("state is not valid JSON (use --state-format text to send it verbatim)")
		}

		// Return the decoded value
		return v, nil
	case "auto", "":
		// Send it as json when it decodes
		if v, ok := tryJSON(text); ok {
			return v, nil
		}

		// Otherwise send the text as is
		return text, nil
	default:
		return nil, fmt.Errorf("unknown state format %q (want auto, json, or text)", format)
	}
}

// tryJSON
// Decodes the text when it looks like a json object or array
// @param text {string} - the text to try
// @return {any, bool}
func tryJSON(text string) (any, bool) {
	// Trim the whitespace
	t := strings.TrimSpace(text)

	// Only objects and arrays count, a bare string or number stays text
	if !strings.HasPrefix(t, "{") && !strings.HasPrefix(t, "[") {
		return nil, false
	}

	// The decoded value
	var v any

	// Decode it
	if err := json.Unmarshal([]byte(t), &v); err != nil {
		return nil, false
	}

	// Return the decoded value
	return v, true
}

// isTerminal
// Says if the reader is an interactive terminal
// @param r {io.Reader} - the reader to check
// @return {bool}
func isTerminal(r io.Reader) bool {
	// Only a file can be a terminal
	f, ok := r.(*os.File)
	if !ok {
		return false
	}

	// Stat it
	fi, err := f.Stat()
	if err != nil {
		return false
	}

	// A char device is a terminal
	return fi.Mode()&os.ModeCharDevice != 0
}
