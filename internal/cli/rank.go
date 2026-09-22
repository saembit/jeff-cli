package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/saembit/jeff-cli/internal/output"
	"github.com/saembit/jeff-cli/internal/rank"
	"github.com/saembit/jeff-cli/pkg/jev"
	"github.com/spf13/cobra"
)

// The flags for the rank command
var rankFlags struct {
	// Print the request instead of sending it
	dryRun bool
}

// The rank command
var rankCmd = &cobra.Command{
	Use:   "rank SPEC.yaml",
	Short: "Score a list of items on weighted dimensions and rank them",
	Long: `rank asks one score question per item per dimension in a single request and
then adds up the scores with the weights from the spec in code, so Jev never
sees the weights and changing one is just a rerun of the arithmetic.

The spec looks like

  context:            # optional, shared facts every question can see
    builder: solo developer, wants revenue in months
  items:              # id to description
    zapier_node: Zapier / n8n node wrapping Jev
    sheets_addon: Sheets add-on exposing Jev as cell functions
  dimensions:         # name to weighted rubric
    revenue:
      weight: 0.3
      question: How much could this earn per year within 18 months?
      levels: [none, under $10k, $10k-50k, $50k-200k, over $200k]
  overall: Which single item makes this builder money soonest?   # optional choice`,
	Example: `  jeff rank examples/rank-ideas.yaml
  jeff rank examples/rank-ideas.yaml -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Open the spec
		fh, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("open spec: %w", err)
		}
		defer fh.Close()

		// Parse and check it
		spec, err := rank.ParseSpec(fh)
		if err != nil {
			return err
		}

		// Build the state and questions
		state, qs := spec.Build()

		// The request
		req := jev.Request{Model: flagModel, State: state, Questions: qs}

		// Print instead of send
		if rankFlags.dryRun {
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

		// Add up the composites
		rows, err := spec.Composite(resp.Answers)
		if err != nil {
			return err
		}

		// The overall choice when the spec asked for one
		overall, hasOverall := resp.Answers[rank.OverallID]

		// Json output
		if f == output.JSON {
			// The rows plus the model and usage
			out := map[string]any{"model": resp.Model, "usage": resp.Usage, "rows": rows}

			// Add the overall when there is one
			if hasOverall {
				out["overall"] = overall
			}

			// Print it
			return output.WriteJSON(cmd.OutOrStdout(), out)
		}

		// Table output
		return writeRankTable(cmd, spec, rows, resp, overall, hasOverall)
	},
}

// writeRankTable
// Prints the ranked rows as a table with the overall choice under it
// @param cmd {*cobra.Command} - the running command
// @param spec {*rank.Spec} - the spec for the dimension names
// @param rows {[]rank.Row} - the ranked rows
// @param resp {*jev.Response} - the response for the model and usage line
// @param overall {jev.Answer} - the overall choice
// @param hasOverall {bool} - whether there was an overall choice
// @return {error}
func writeRankTable(cmd *cobra.Command, spec *rank.Spec, rows []rank.Row, resp *jev.Response, overall jev.Answer, hasOverall bool) error {
	// A writer that lines up the columns
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	// The dimension names in a stable order
	dims := spec.DimensionNames()

	// The header row
	fmt.Fprintf(w, "#\titem\tcomposite\t%s\n", strings.Join(dims, "\t"))

	// One line per row
	for i, r := range rows {
		// The score per dimension
		cells := make([]string, len(dims))
		for j, d := range dims {
			cells[j] = fmt.Sprintf("%.2f", r.Scores[d])
		}

		// Print the line
		fmt.Fprintf(w, "%d\t%s\t%.2f\t%s\n", i+1, r.ID, r.Composite, strings.Join(cells, "\t"))
	}

	// Flush the table
	if err := w.Flush(); err != nil {
		return err
	}

	// Print the overall when there is one
	if hasOverall {
		// The pick and its confidence
		fmt.Fprintf(cmd.OutOrStdout(), "\noverall: %s (confidence %.2f)\n", overall.Choice, deref(overall.Confidence))

		// The options highest first
		keys := make([]string, 0, len(overall.Probabilities))
		for k := range overall.Probabilities {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return overall.Probabilities[keys[i]] > overall.Probabilities[keys[j]] })

		// Print the top five that have any weight
		for i, k := range keys {
			if i >= 5 || overall.Probabilities[k] < 0.01 {
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %.3f\n", k, overall.Probabilities[k])
		}
	}

	// The model and usage line
	fmt.Fprintf(cmd.OutOrStdout(), "model %s · %d input tokens · %d questions\n", resp.Model, resp.Usage.InputTokens, len(resp.Answers))

	// Done
	return nil
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

func init() {
	// The rank flags
	rankCmd.Flags().BoolVar(&rankFlags.dryRun, "dry-run", false, "print the request body and send nothing")

	// Hang it off the root
	rootCmd.AddCommand(rankCmd)
}
