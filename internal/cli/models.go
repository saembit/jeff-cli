package cli

import (
	"fmt"

	"github.com/saembit/jeff-cli/internal/output"
	"github.com/spf13/cobra"
)

// The models command
var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List the models and aliases this account can use",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Fetch the list
		models, err := c.Models(cmdContext(cmd))
		if err != nil {
			return err
		}

		// Json output
		if f == output.JSON {
			return output.WriteJSON(cmd.OutOrStdout(), models)
		}

		// One line per model
		for _, m := range models {
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %s  %s\n", m.Name, m.ReleaseDate, m.Description)
		}

		// Done
		return nil
	},
}

func init() {
	// Hang it off the root
	rootCmd.AddCommand(modelsCmd)
}
