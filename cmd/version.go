package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/supermetrics-public/supermetrics-cli/internal/buildcfg"
	"github.com/supermetrics-public/supermetrics-cli/internal/update"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(infoWriter(), "supermetrics-cli %s (built %s, commit %s)\n", buildcfg.Version, buildcfg.BuildDate, buildcfg.Commit)
		update.PrintUpdateHint(infoWriterErr(), buildcfg.Version)
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade to the latest version",
	Long:  `Download and install the latest version of the Supermetrics CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		w := infoWriter()
		u := update.NewUpdater()

		checkOnly, _ := cmd.Flags().GetBool("check")
		if checkOnly {
			return u.CheckOnly(ctx, w, buildcfg.Version)
		}

		force, _ := cmd.Flags().GetBool("force")
		if force {
			return u.ForceReinstall(ctx, w, buildcfg.Version)
		}

		return u.Upgrade(ctx, w, buildcfg.Version)
	},
}

func init() {
	upgradeCmd.Flags().Bool("check", false, "Only check for updates, don't install")
	upgradeCmd.Flags().Bool("force", false, "Force reinstall of current version")

	versionCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(versionCmd)
}
