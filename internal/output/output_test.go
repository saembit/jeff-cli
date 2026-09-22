package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/saembit/jeff-cli/pkg/jev"
)

// Checks the table has a line for each answer type and the model line
func TestWriteAnswersTable(t *testing.T) {
	// A helper to take a float's address
	f := func(v float64) *float64 { return &v }

	// A response with one of each type
	resp := &jev.Response{Model: "m", Answers: map[string]jev.Answer{
		"u": {Type: "noul", Noul: f(0.5)},
		"t": {Type: "choice", Choice: "a", Probabilities: map[string]float64{"a": 0.7, "b": 0.3}, Confidence: f(0.4)},
		"s": {Type: "score", Score: f(1.5), Legend: map[string]string{"0": "lo", "1": "hi"}, Probabilities: map[string]float64{"0": 0.5, "1": 0.5}, Confidence: f(0)},
	}}

	// Print it
	var b bytes.Buffer
	if err := WriteAnswers(&b, Table, resp); err != nil {
		t.Fatal(err)
	}

	// Each piece shows up
	out := b.String()
	for _, want := range []string{"u ", "0.500", "choice a", "score 1.50", "hi", "model m"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// Checks the format flag ignores case and refuses unknown names
func TestParseFormat(t *testing.T) {
	// Upper case json works
	if f, _ := ParseFormat("JSON"); f != JSON {
		t.Error("JSON not parsed")
	}

	// xml is refused
	if _, err := ParseFormat("xml"); err == nil {
		t.Error("xml accepted")
	}
}

// Checks the bar clamps to the ends
func TestBarClamps(t *testing.T) {
	// Over 1 is full and under 0 is empty
	if bar(2) != strings.Repeat("█", barWidth) || bar(-1) != strings.Repeat("░", barWidth) {
		t.Error("bar not clamped")
	}
}
