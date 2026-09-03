// Command cinc is a unified command-line tool for Cinc/Chef Infra.
package main

import (
	"os"

	"github.com/cinc-project/cinc-cli/apps/cinc/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
