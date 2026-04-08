package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/supermetrics-public/supermetrics-cli/internal/config"
	"github.com/supermetrics-public/supermetrics-cli/internal/exitcode"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage named credential profiles",
	Long: `Manage named credential profiles for switching between API keys or OAuth accounts.

Profiles are created implicitly by running 'supermetrics configure --profile <name>'
or 'supermetrics login --profile <name>'.`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if len(cfg.Profiles) == 0 {
			fmt.Fprintln(infoWriter(), "No profiles configured. Run 'supermetrics configure' to create one.")
			return nil
		}

		active := cfg.ActiveOrDefault()
		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)

		w := cmd.OutOrStdout()
		for _, name := range names {
			p := cfg.Profiles[name]
			marker := "  "
			if name == active {
				marker = "* "
			}

			authType := "no credentials"
			if p.AccessToken != "" {
				if p.IsTokenExpired() {
					authType = "OAuth (expired)"
				} else {
					authType = "OAuth"
				}
			} else if p.APIKey != "" {
				authType = "API key"
			}

			fmt.Fprintf(w, "%s%s (%s)\n", marker, name, authType)
		}
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load()
		if err != nil {
			cfg = &config.Config{}
		}

		// Verify profile exists
		if cfg.Profiles == nil || cfg.Profiles[name] == nil {
			return exitcode.Wrap(
				fmt.Errorf("profile %q not found. Create it with 'supermetrics configure --profile %s'", name, name),
				exitcode.Usage,
			)
		}

		cfg.ActiveProfile = name
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Fprintf(infoWriter(), "Active profile set to %q.\n", name)
		return nil
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if cfg.Profiles == nil || cfg.Profiles[name] == nil {
			return exitcode.Wrap(
				fmt.Errorf("profile %q not found", name),
				exitcode.Usage,
			)
		}

		delete(cfg.Profiles, name)

		// Clear active profile if it was the deleted one
		if cfg.ActiveProfile == name {
			cfg.ActiveProfile = ""
		}

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Fprintf(infoWriter(), "Profile %q deleted.\n", name)
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show profile details",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		name := GetProfile()
		if len(args) > 0 {
			name = args[0]
		}

		if cfg.Profiles == nil || cfg.Profiles[name] == nil {
			return exitcode.Wrap(
				fmt.Errorf("profile %q not found", name),
				exitcode.Usage,
			)
		}

		p := cfg.Profiles[name]
		w := cmd.OutOrStdout()

		fmt.Fprintf(w, "Profile: %s\n", name)
		if name == cfg.ActiveOrDefault() {
			fmt.Fprintln(w, "Status:  active")
		}
		fmt.Fprintf(w, "API key: %s\n", maskKey(p.APIKey))

		if p.AccessToken != "" {
			if p.IsTokenExpired() {
				fmt.Fprintln(w, "OAuth:   expired")
			} else {
				fmt.Fprintln(w, "OAuth:   authenticated")
			}
		} else {
			fmt.Fprintln(w, "OAuth:   not logged in")
		}
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileShowCmd)
	rootCmd.AddCommand(profileCmd)
}
