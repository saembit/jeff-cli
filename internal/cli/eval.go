package cli

import (
	"fmt"
	"os"

	"github.com/saembit/jeff-cli/internal/output"
	"github.com/saembit/jeff-cli/pkg/jev"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// The flags for the eval command
var evalFlags struct {
	stateFlags
	// The request file
	file string
	// Print the request instead of sending it
	dryRun bool
}

// requestFile is the shape of the request file, state and model are optional
type requestFile struct {
	// The model to send
	Model string `yaml:"model"`
	// The state when the file carries it
	State any `yaml:"state"`
	// The questions keyed by id
	Questions map[string]jev.Question `yaml:"questions"`
}

// The eval command
var evalCmd = &cobra.Command{
	Use:   "eval -f REQUEST.yaml",
	Short: "Ask a file full of questions about one state",
	Example: `  jeff eval -f triage.yaml --state-file ticket.txt
  jeff eval -f triage.yaml --state-file ticket.txt -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read the request file
		b, err := os.ReadFile(evalFlags.file)
		if err != nil {
			return fmt.Errorf("read request file: %w", err)
		}

		// The decoded file
		var rf requestFile

		// Decode it, yaml is a superset of json so both work
		if err := yaml.Unmarshal(b, &rf); err != nil {
			return fmt.Errorf("parse request file: %w", err)
		}

		// A file with no questions is useless
		if len(rf.Questions) == 0 {
			return fmt.Errorf("request file has no questions")
		}

		// Start with the state from the file
		state := rf.State

		// The flags win over the file
		if evalFlags.text != "" || evalFlags.stateFlags.file != "" {
			state, err = evalFlags.resolve()
			if err != nil {
				return err
			}
		}

		// Still no state
		if state == nil {
			return fmt.Errorf("no state: put one in the file or use --state / --state-file")
		}

		// Start with the model from the file
		model := rf.Model

		// The flag wins when set or the file left it blank
		if cmd.Flags().Changed("model") || model == "" {
			model = flagModel
		}

		// The request with the yaml maps turned into json maps
		req := jev.Request{Model: model, State: normalizeYAML(state), Questions: normalizeQuestions(rf.Questions)}

		// Print instead of send
		if evalFlags.dryRun {
			return output.WriteJSON(cmd.OutOrStdout(), req)
		}

		// Check the output format
		f, err := outputFormat()
		if err != nil {
			return err
		}

		// Build the client
		c, err := newClient()
		if err != nil {
			return err
		}

		// Send it
		resp, err := c.Do(cmdContext(cmd), req)
		if err != nil {
			return err
		}

		// Print the answers
		return output.WriteAnswers(cmd.OutOrStdout(), f, resp)
	},
}

// normalizeQuestions
// Runs normalizeYAML over the instructions and criteria of every question
// @param qs {map[string]jev.Question} - the questions from the file
// @return {map[string]jev.Question}
func normalizeQuestions(qs map[string]jev.Question) map[string]jev.Question {
	// The cleaned copy
	out := make(map[string]jev.Question, len(qs))

	// Clean each question
	for id, q := range qs {
		out[id] = jev.Question{Type: q.Type, Instructions: normalizeYAML(q.Instructions), Criteria: normalizeYAML(q.Criteria)}
	}

	// Return the copy
	return out
}

// normalizeYAML
// Turns the map[any]any that yaml decodes into map[string]any so json can encode it
// @param v {any} - the decoded yaml value
// @return {any}
func normalizeYAML(v any) any {
	// Handle each shape
	switch t := v.(type) {
	case map[any]any:
		// Convert every key to a string
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = normalizeYAML(val)
		}
		return m
	case map[string]any:
		// Clean the values
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = normalizeYAML(val)
		}
		return m
	case []any:
		// Clean each element
		s := make([]any, len(t))
		for i, val := range t {
			s[i] = normalizeYAML(val)
		}
		return s
	default:
		// Scalars pass through
		return v
	}
}

func init() {
	// The eval flags
	evalFlags.stateFlags.bind(evalCmd)
	evalCmd.Flags().StringVarP(&evalFlags.file, "file", "f", "", "request file, json or yaml, with questions and optionally state and model")
	evalCmd.Flags().BoolVar(&evalFlags.dryRun, "dry-run", false, "print the request body and send nothing")
	_ = evalCmd.MarkFlagRequired("file")

	// Hang it off the root
	rootCmd.AddCommand(evalCmd)
}
