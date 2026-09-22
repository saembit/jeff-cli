// Package output prints answers as json or a table for people
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/saembit/jeff-cli/pkg/jev"
)

// Format is an output format name
type Format string

const (
	// JSON prints the response as is
	JSON Format = "json"
	// Table prints a readable table with bars
	Table Format = "table"
)

// The width of a probability bar in chars
const barWidth = 20

// ParseFormat
// Checks a format name from the flag
// @param s {string} - the name given
// @return {Format, error}
func ParseFormat(s string) (Format, error) {
	// Match it ignoring case
	switch Format(strings.ToLower(s)) {
	case JSON:
		return JSON, nil
	case Table, "":
		return Table, nil
	}

	// Not a format we know
	return "", fmt.Errorf("unknown output format %q (want json or table)", s)
}

// WriteJSON
// Pretty prints any value as json
// @param w {io.Writer} - where to write
// @param v {any} - the value to print
// @return {error}
func WriteJSON(w io.Writer, v any) error {
	// The encoder with two space indent
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	// Write it
	return enc.Encode(v)
}

// WriteAnswers
// Prints a response in the format given
// @param w {io.Writer} - where to write
// @param f {Format} - json or table
// @param resp {*jev.Response} - the response to print
// @return {error}
func WriteAnswers(w io.Writer, f Format, resp *jev.Response) error {
	// Json is just the response
	if f == JSON {
		return WriteJSON(w, resp)
	}

	// The answer ids sorted so output is stable
	ids := make([]string, 0, len(resp.Answers))
	for id := range resp.Answers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Print each answer
	for _, id := range ids {
		writeAnswer(w, id, resp.Answers[id])
	}

	// Print the model and token line
	fmt.Fprintf(w, "model %s · %d input tokens\n", resp.Model, resp.Usage.InputTokens)

	// Done
	return nil
}

// writeAnswer
// Prints one answer in the shape of its type
// @param w {io.Writer} - where to write
// @param id {string} - the question id
// @param a {jev.Answer} - the answer
func writeAnswer(w io.Writer, id string, a jev.Answer) {
	// Print by type
	switch a.Type {
	case "noul":
		// The probability and a bar
		fmt.Fprintf(w, "%-16s noul  %.3f %s\n", id, deref(a.Noul), bar(deref(a.Noul)))
	case "choice":
		// The pick and confidence then each option highest first
		fmt.Fprintf(w, "%-16s choice %s  (confidence %.2f)\n", id, a.Choice, deref(a.Confidence))
		for _, k := range sortedByProb(a.Probabilities) {
			fmt.Fprintf(w, "  %-22s %.3f %s\n", k, a.Probabilities[k], bar(a.Probabilities[k]))
		}
	case "score":
		// The score and confidence then each level in order
		fmt.Fprintf(w, "%-16s score %.2f  (confidence %.2f)\n", id, deref(a.Score), deref(a.Confidence))
		for _, k := range sortedKeys(a.Probabilities) {
			fmt.Fprintf(w, "  %s %-20s %.3f %s\n", k, trunc(a.Legend[k], 20), a.Probabilities[k], bar(a.Probabilities[k]))
		}
	default:
		// A type this version doesn't know
		fmt.Fprintf(w, "%-16s %s (unknown answer type)\n", id, a.Type)
	}
}

// bar
// Draws a probability as a filled bar
// @param p {float64} - the probability
// @return {string}
func bar(p float64) string {
	// Clamp to 0..1
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}

	// How many chars are filled
	n := int(p*barWidth + 0.5)

	// Return the filled and empty parts
	return strings.Repeat("█", n) + strings.Repeat("░", barWidth-n)
}

// deref
// Reads a float pointer as 0 when nil
// @param f {*float64} - the pointer
// @return {float64}
func deref(f *float64) float64 {
	// Nil reads as 0
	if f == nil {
		return 0
	}

	// Return the value
	return *f
}

// trunc
// Cuts a string to n chars with an ellipsis
// @param s {string} - the string
// @param n {int} - the max length
// @return {string}
func trunc(s string, n int) string {
	// Short enough
	if len(s) <= n {
		return s
	}

	// Cut it and mark it
	return s[:n-1] + "…"
}

// sortedKeys
// Gives the map keys sorted
// @param m {map[string]float64} - the map
// @return {[]string}
func sortedKeys(m map[string]float64) []string {
	// The keys
	ks := make([]string, 0, len(m))

	// Collect them
	for k := range m {
		ks = append(ks, k)
	}

	// Sort them
	sort.Strings(ks)

	// Return them
	return ks
}

// sortedByProb
// Gives the map keys highest probability first
// @param m {map[string]float64} - the map
// @return {[]string}
func sortedByProb(m map[string]float64) []string {
	// Start sorted by name so ties are stable
	ks := sortedKeys(m)

	// Sort by probability
	sort.SliceStable(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] })

	// Return them
	return ks
}
