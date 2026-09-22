package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// resetFlags
// Puts every flag on the tree back to its default so tests don't leak state through the package level flag vars
// @param c {*cobra.Command} - the root of the tree
func resetFlags(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}

// fakeServer
// Starts a server that answers every call with body and keeps the last request
// @param t {*testing.T} - the test
// @param body {string} - the json to answer with
// @return {*httptest.Server, *map[string]any}
func fakeServer(t *testing.T, body string) (*httptest.Server, *map[string]any) {
	t.Helper()
	var last map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&last)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

// run
// Runs the root command against the fake server and captures the output
// @param t {*testing.T} - the test
// @param srv {*httptest.Server} - the fake server
// @param args {...string} - the command line
// @return {string, error}
func run(t *testing.T, srv *httptest.Server, args ...string) (string, error) {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", "test")
	resetFlags(rootCmd)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(append([]string{"--base-url", srv.URL}, args...))
	err := rootCmd.Execute()
	return out.String(), err
}

// Checks the state goes out, the answer prints and the gate holds
func TestNoulGatePasses(t *testing.T) {
	srv, last := fakeServer(t, `{"model":"m","answers":{"answer":{"type":"noul","noul":0.9}},"usage":{}}`)
	out, err := run(t, srv, "noul", "urgent?", "--state", "help now", "--fail-under", "0.7")
	if err != nil {
		t.Fatalf("err = %v, out = %s", err, out)
	}
	if (*last)["state"] != "help now" {
		t.Errorf("state = %v", (*last)["state"])
	}
	if !strings.Contains(out, "0.900") {
		t.Errorf("out = %s", out)
	}
}

// Checks a failed gate comes back as the gate error
func TestNoulGateFailsWithExitCodeError(t *testing.T) {
	srv, _ := fakeServer(t, `{"model":"m","answers":{"answer":{"type":"noul","noul":0.2}},"usage":{}}`)
	_, err := run(t, srv, "noul", "urgent?", "--state", "meh", "--fail-under", "0.7")
	if !errors.Is(err, errGateFailed) {
		t.Fatalf("err = %v, want gate failure", err)
	}
}

// Checks a threshold over 1 is refused before anything is sent
func TestNoulRejectsThresholdOutOfRange(t *testing.T) {
	srv, _ := fakeServer(t, `{}`)
	_, err := run(t, srv, "noul", "q", "--state", "x", "--fail-under", "2")
	if err == nil || !strings.Contains(err.Error(), "between 0 and 1") {
		t.Fatalf("err = %v", err)
	}
}

// Checks a response missing our answer is an error
func TestMissingAnswerIsError(t *testing.T) {
	srv, _ := fakeServer(t, `{"model":"m","answers":{},"usage":{}}`)
	_, err := run(t, srv, "noul", "q", "--state", "x")
	if err == nil || !strings.Contains(err.Error(), "no answer") {
		t.Fatalf("err = %v", err)
	}
}

// Checks NAME=DESC and bare NAME options both go out right
func TestChoiceOptionsParsed(t *testing.T) {
	srv, last := fakeServer(t, `{"model":"m","answers":{"answer":{"type":"choice","choice":"a","probabilities":{"a":1},"confidence":1}},"usage":{}}`)
	if _, err := run(t, srv, "choice", "which?", "--state", "x", "--option", "a=first", "--option", "b"); err != nil {
		t.Fatal(err)
	}
	crit := (*last)["questions"].(map[string]any)["answer"].(map[string]any)["criteria"].(map[string]any)
	if crit["a"] != "first" || crit["b"] != nil {
		t.Errorf("criteria = %v", crit)
	}
}

// Checks yaml criteria come out as json maps and lists in the dry run
func TestEvalDryRunNormalizesYAML(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "q.yaml")
	_ = os.WriteFile(spec, []byte(`
questions:
  urgent:
    type: noul
    instructions: urgent?
    criteria: {"true": yes it is, "false": no}
  mood:
    type: score
    instructions: mood?
    criteria: [calm, mad]
`), 0o644)
	srv, _ := fakeServer(t, `{}`)
	out, err := run(t, srv, "eval", "-f", spec, "--state", "hi", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var req map[string]any
	if err := json.Unmarshal([]byte(out), &req); err != nil {
		t.Fatalf("dry-run output is not JSON: %s", out)
	}
	qs := req["questions"].(map[string]any)
	if qs["urgent"].(map[string]any)["criteria"].(map[string]any)["true"] != "yes it is" {
		t.Errorf("criteria = %v", qs["urgent"])
	}
	if len(qs["mood"].(map[string]any)["criteria"].([]any)) != 2 {
		t.Errorf("levels = %v", qs["mood"])
	}
}
