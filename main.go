package main

import (
	"os"

	"github.com/supermetrics-public/supermetrics-cli/cmd"
	"github.com/supermetrics-public/supermetrics-cli/internal/exitcode"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(exitcode.Of(err))
	}
}
