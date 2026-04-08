package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manCmd = &cobra.Command{
	Use:   "man [directory]",
	Short: "Generate man pages",
	Long:  `Generate man pages for all commands and write them to the specified directory.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := args[0]
		header := &doc.GenManHeader{
			Title:   "SUPERMETRICS",
			Section: "1",
		}
		if err := doc.GenManTree(rootCmd, header, dir); err != nil {
			return fmt.Errorf("failed to generate man pages: %w", err)
		}
		fmt.Fprintf(infoWriter(), "Man pages written to %s\n", dir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}
