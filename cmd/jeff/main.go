// Command jeff asks TypeSafe's Jev model typed questions from the terminal
package main

import (
	"os"

	"github.com/saembit/jeff-cli/internal/cli"
)

func main() {
	// Run the cli and exit with its code
	os.Exit(cli.Execute())
}
