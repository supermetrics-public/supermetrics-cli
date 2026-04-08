package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/supermetrics-public/supermetrics-cli/internal/config"
	"github.com/supermetrics-public/supermetrics-cli/internal/exitcode"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure the Supermetrics CLI",
	Long:  `Interactively set up your API key and default settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			cfg = &config.Config{}
		}

		profileName := GetProfile()
		profile := cfg.GetProfile(profileName)

		reader := bufio.NewReader(os.Stdin)

		// API key
		current := maskKey(profile.APIKey)
		fmt.Printf("API key [%s]: ", current)
		input, err := readLine(reader)
		if err != nil {
			return err
		}
		if input != "" {
			profile.APIKey = input
		}

		// Default output format
		if cfg.DefaultOutput == "" {
			cfg.DefaultOutput = "json"
		}
		fmt.Printf("Default output format (json/table/csv) [%s]: ", cfg.DefaultOutput)
		input, err = readLine(reader)
		if err != nil {
			return err
		}
		if input != "" {
			cfg.DefaultOutput = input
		}

		if err := cfg.Validate(); err != nil {
			return exitcode.Wrap(fmt.Errorf("invalid configuration: %w", err), exitcode.Usage)
		}

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		p, _ := config.Path()
		if profileName != config.DefaultProfile {
			fmt.Fprintf(infoWriter(), "Configuration saved to %s (profile: %s)\n", p, profileName)
		} else {
			fmt.Fprintf(infoWriter(), "Configuration saved to %s\n", p)
		}
		return nil
	},
}

// readLine reads a line from the reader, trimming whitespace.
// Returns empty string on EOF (user pressed enter or input ended).
func readLine(reader *bufio.Reader) (string, error) {
	input, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(input), nil
}

func maskKey(key string) string {
	if key == "" {
		return "not set"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func init() {
	rootCmd.AddCommand(configureCmd)
}
