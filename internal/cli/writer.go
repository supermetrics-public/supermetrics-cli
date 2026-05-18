package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"
)

// IsQuiet returns whether quiet mode is enabled via --quiet flag or SUPERMETRICS_QUIET env var.
func IsQuiet(cmd *cobra.Command) bool {
	if q, _ := cmd.Root().PersistentFlags().GetBool("quiet"); q {
		return true
	}
	if val, ok := os.LookupEnv("SUPERMETRICS_QUIET"); ok && val != "" && val != "0" {
		return true
	}
	return false
}

// InfoWriter returns a writer for informational stdout messages.
// Returns io.Discard in quiet mode.
func InfoWriter(cmd *cobra.Command) io.Writer {
	if IsQuiet(cmd) {
		return io.Discard
	}
	return cmd.OutOrStdout()
}

// InfoWriterErr returns a writer for informational stderr messages.
// Returns io.Discard in quiet mode.
func InfoWriterErr(cmd *cobra.Command) io.Writer {
	if IsQuiet(cmd) {
		return io.Discard
	}
	return cmd.ErrOrStderr()
}
