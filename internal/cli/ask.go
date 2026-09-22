package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/saembit/jeff-cli/internal/input"
	"github.com/saembit/jeff-cli/internal/output"
	"github.com/saembit/jeff-cli/pkg/jev"
	"github.com/spf13/cobra"
)

// stateFlags are the flags every command that reads one state shares
type stateFlags struct {
	// The --state flag
	text string
	// The --state-file flag
	file string
	// The --state-format flag
	format string
}

// bind
// Adds the state flags to a command
// @param cmd {*cobra.Command} - the command to add them to
func (s *stateFlags) bind(cmd *cobra.Command) {
	// Register the three flags
	cmd.Flags().StringVar(&s.text, "state", "", "the state to evaluate, as text")
	cmd.Flags().StringVar(&s.file, "state-file", "", "read the state from a file, or stdin with -")
	cmd.Flags().StringVar(&s.format, "state-format", "auto", "auto, json, or text")
}

// resolve
// Reads the state from the flags or stdin and parses it
// @return {any, error}
func (s *stateFlags) resolve() (any, error) {
	// Read the text from whichever source was given
	text, err := input.ReadState(s.text, s.file, os.Stdin)
	if err != nil {
		return nil, err
	}

	// Parse it as json or text
	return input.ParseState(text, s.format)
}

// runSingle
// Sends one question, prints the answer and runs the gate on it
// @param cmd {*cobra.Command} - the running command
// @param sf {*stateFlags} - the state flags
// @param id {string} - the question id
// @param q {jev.Question} - the question
// @param gate {func(jev.Answer) error} - the threshold check, nil for none
// @return {error}
func runSingle(cmd *cobra.Command, sf *stateFlags, id string, q jev.Question, gate func(jev.Answer) error) error {
	// Read the state
	state, err := sf.resolve()
	if err != nil {
		return err
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

	// Send the question
	resp, err := c.SystemOne(cmdContext(cmd), state, map[string]jev.Question{id: q})
	if err != nil {
		return err
	}

	// Print the answer
	if err := output.WriteAnswers(cmd.OutOrStdout(), f, resp); err != nil {
		return err
	}

	// The answer for our id
	a, ok := resp.Answers[id]

	// The api should always answer the id we sent
	if !ok {
		return fmt.Errorf("response has no answer for %q", id)
	}

	// Run the gate when there is one
	if gate != nil {
		return gate(a)
	}

	// Done
	return nil
}

// probability
// Checks a threshold flag is between 0 and 1
// @param name {string} - the flag name for the error
// @param v {float64} - the value given
// @return {error}
func probability(name string, v float64) error {
	// Out of range
	if v < 0 || v > 1 {
		return fmt.Errorf("--%s must be between 0 and 1, got %g", name, v)
	}

	// Fine
	return nil
}

// The flags for the noul command
var noulFlags struct {
	stateFlags
	// What a yes and a no mean
	yes, no string
	// The gates on P(yes)
	failUnder, failOver float64
}

// The noul command
var noulCmd = &cobra.Command{
	Use:   "noul QUESTION",
	Short: "Ask a yes/no question and get P(yes)",
	Example: `  jeff noul "Is the customer asking for a refund?" --state-file ticket.txt
  git log -1 --pretty=%B | jeff noul "Does this describe a user-facing change?" --fail-under 0.7`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// The criteria when either meaning was given
		var crit *jev.NoulCriteria
		if noulFlags.yes != "" || noulFlags.no != "" {
			crit = &jev.NoulCriteria{True: noulFlags.yes, False: noulFlags.no}
		}

		// The question
		q := jev.Noul(args[0], crit)

		// Check the gate flags are probabilities
		if err := probability("fail-under", noulFlags.failUnder); err != nil {
			return err
		}
		if err := probability("fail-over", noulFlags.failOver); err != nil {
			return err
		}

		// The gate on P(yes)
		gate := func(a jev.Answer) error {
			// A noul answer has to carry a value
			if a.Noul == nil {
				return fmt.Errorf("answer has no noul value")
			}

			// P(yes)
			p := *a.Noul

			// Fail when under the floor
			if cmd.Flags().Changed("fail-under") && p < noulFlags.failUnder {
				return fmt.Errorf("%w: P(yes)=%.3f < %.3f", errGateFailed, p, noulFlags.failUnder)
			}

			// Fail when over the ceiling
			if cmd.Flags().Changed("fail-over") && p > noulFlags.failOver {
				return fmt.Errorf("%w: P(yes)=%.3f > %.3f", errGateFailed, p, noulFlags.failOver)
			}

			// Held
			return nil
		}

		// Send it
		return runSingle(cmd, &noulFlags.stateFlags, "answer", q, gate)
	},
}

// The flags for the choice command
var choiceFlags struct {
	stateFlags
	// The options as NAME or NAME=DESCRIPTION
	options []string
	// The gate on confidence
	minConfidence float64
}

// The choice command
var choiceCmd = &cobra.Command{
	Use:   "choice QUESTION --option NAME[=DESCRIPTION] ...",
	Short: "Pick one option and get a probability per option",
	Example: `  jeff choice "Which team handles this?" --state-file ticket.txt \
      --option billing="payments, refunds" --option technical="bugs, outages" --option other`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// A choice needs at least two options
		if len(choiceFlags.options) < 2 {
			return fmt.Errorf("need at least two --option values")
		}

		// The options as the api wants them
		opts := make(map[string]any, len(choiceFlags.options))

		// Split each option on the first =
		for _, o := range choiceFlags.options {
			name, desc, has := strings.Cut(o, "=")
			if has {
				opts[name] = desc
			} else {
				opts[name] = nil
			}
		}

		// Check the gate flag is a probability
		if err := probability("min-confidence", choiceFlags.minConfidence); err != nil {
			return err
		}

		// The gate on confidence
		gate := func(a jev.Answer) error {
			// A choice answer has to carry a confidence
			if a.Confidence == nil {
				return fmt.Errorf("answer has no confidence value")
			}

			// Fail when under the floor
			if cmd.Flags().Changed("min-confidence") && *a.Confidence < choiceFlags.minConfidence {
				return fmt.Errorf("%w: confidence %.3f < %.3f", errGateFailed, *a.Confidence, choiceFlags.minConfidence)
			}

			// Held
			return nil
		}

		// Send it
		return runSingle(cmd, &choiceFlags.stateFlags, "answer", jev.Choice(args[0], opts), gate)
	},
}

// The flags for the score command
var scoreFlags struct {
	stateFlags
	// The level descriptions lowest first
	levels []string
	// The gate on the score
	failUnder float64
}

// The score command
var scoreCmd = &cobra.Command{
	Use:   "score QUESTION --level DESC --level DESC ...",
	Short: "Rate the state on ordered levels and get a weighted score",
	Example: `  jeff score "How frustrated is the customer?" --state-file ticket.txt \
      --level Calm --level Frustrated --level "Very angry"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// A score needs at least two levels
		if len(scoreFlags.levels) < 2 {
			return fmt.Errorf("need at least two --level values")
		}

		// The levels as the api wants them
		levels := make([]any, len(scoreFlags.levels))
		for i, l := range scoreFlags.levels {
			levels[i] = l
		}

		// The gate on the score
		gate := func(a jev.Answer) error {
			// A score answer has to carry a value
			if a.Score == nil {
				return fmt.Errorf("answer has no score value")
			}

			// Fail when under the floor
			if cmd.Flags().Changed("fail-under") && *a.Score < scoreFlags.failUnder {
				return fmt.Errorf("%w: score %.2f < %.2f", errGateFailed, *a.Score, scoreFlags.failUnder)
			}

			// Held
			return nil
		}

		// Send it
		return runSingle(cmd, &scoreFlags.stateFlags, "answer", jev.Score(args[0], levels), gate)
	},
}

func init() {
	// The noul flags
	noulFlags.bind(noulCmd)
	noulCmd.Flags().StringVar(&noulFlags.yes, "yes", "", "what a yes means")
	noulCmd.Flags().StringVar(&noulFlags.no, "no", "", "what a no means")
	noulCmd.Flags().Float64Var(&noulFlags.failUnder, "fail-under", 0, "exit 10 when P(yes) is below this")
	noulCmd.Flags().Float64Var(&noulFlags.failOver, "fail-over", 0, "exit 10 when P(yes) is above this")

	// The choice flags
	choiceFlags.bind(choiceCmd)
	choiceCmd.Flags().StringArrayVar(&choiceFlags.options, "option", nil, "an option as NAME or NAME=DESCRIPTION, repeat for each")
	choiceCmd.Flags().Float64Var(&choiceFlags.minConfidence, "min-confidence", 0, "exit 10 when confidence is below this")

	// The score flags
	scoreFlags.bind(scoreCmd)
	scoreCmd.Flags().StringArrayVar(&scoreFlags.levels, "level", nil, "a level description lowest first, repeat for each")
	scoreCmd.Flags().Float64Var(&scoreFlags.failUnder, "fail-under", 0, "exit 10 when the score is below this")

	// Hang the three commands off the root
	rootCmd.AddCommand(noulCmd, choiceCmd, scoreCmd)
}
