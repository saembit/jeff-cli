// Package cli holds the jeff command tree
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/saembit/jeff-cli/internal/output"
	"github.com/saembit/jeff-cli/pkg/jev"
	"github.com/spf13/cobra"
)

const (
	// ExitOK is returned when everything worked and every gate held
	ExitOK = 0
	// ExitError is returned for bad flags, network and api errors
	ExitError = 1
	// ExitGateFail is returned when the request worked but a threshold didn't hold
	ExitGateFail = 10
)

var (
	// The model alias or id to send
	flagModel string
	// The output format, table or json
	flagOutput string
	// The api root
	flagBaseURL string
	// Time allowed per attempt
	flagTimeout time.Duration
	// Retries after the first attempt
	flagMaxRetries int
)

// errGateFailed is returned by a gate so Execute can map it to exit 10
var errGateFailed = errors.New("gate failed")

// The root command every other command hangs off
var rootCmd = &cobra.Command{
	Use:   "jeff",
	Short: "Ask Jev a question from the terminal and get a number back",
	Long: `jeff sends some state and a typed question to TypeSafe's Jev model and prints
the probabilities it gives back. Jev doesn't write text, you define the answers
and it tells you how likely each one is, so the exit code can gate a shell
script or CI job, the json output can feed an agent, and jeff rank can score a
list of things on a few weighted dimensions in one call.

Needs TYPESAFE_API_KEY in the environment.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// The flags every command shares
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagModel, "model", jev.DefaultModel, "model alias or versioned id")
	pf.StringVarP(&flagOutput, "output", "o", "table", "output format, table or json")
	pf.StringVar(&flagBaseURL, "base-url", jev.DefaultBaseURL, "api root")
	pf.DurationVar(&flagTimeout, "timeout", 30*time.Second, "time allowed per attempt")
	pf.IntVar(&flagMaxRetries, "max-retries", 2, "retries after the first attempt on 429, 529 and 5xx")
}

// Execute
// Runs the cli and turns the result into an exit code
// @return {int}
func Execute() int {
	// Run the command tree
	err := rootCmd.Execute()

	// Map the result to an exit code
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, errGateFailed):
		return ExitGateFail
	default:
		// Print the error and fail
		fmt.Fprintln(os.Stderr, "jeff:", err)
		return ExitError
	}
}

// newClient
// Builds an api client from the global flags
// @return {*jev.Client, error}
func newClient() (*jev.Client, error) {
	// Return a client with every global flag applied
	return jev.New(
		jev.WithModel(flagModel),
		jev.WithBaseURL(flagBaseURL),
		jev.WithTimeout(flagTimeout),
		jev.WithMaxRetries(flagMaxRetries),
	)
}

// outputFormat
// Checks the output flag
// @return {output.Format, error}
func outputFormat() (output.Format, error) { return output.ParseFormat(flagOutput) }

// cmdContext
// Gives the command's context or a background one
// @param cmd {*cobra.Command} - the running command
// @return {context.Context}
func cmdContext(cmd *cobra.Command) context.Context {
	// Use the command's context when it has one
	if cmd.Context() != nil {
		return cmd.Context()
	}

	// Fall back to a background context
	return context.Background()
}
