package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/supermetrics-public/supermetrics-cli/cmd/generated"
	"github.com/supermetrics-public/supermetrics-cli/internal/auth"
	"github.com/supermetrics-public/supermetrics-cli/internal/buildcfg"
	"github.com/supermetrics-public/supermetrics-cli/internal/config"
	"github.com/supermetrics-public/supermetrics-cli/internal/exitcode"
	"github.com/supermetrics-public/supermetrics-cli/internal/httpclient"
	"github.com/supermetrics-public/supermetrics-cli/internal/output"
	"github.com/supermetrics-public/supermetrics-cli/internal/update"
)

var (
	flagAPIKey  string
	flagOutput  string
	flagVerbose bool
	flagNoColor bool
	flagFlatten bool
	flagNoRetry bool
	flagQuiet   bool
	flagFields  string
	flagProfile string
	flagTimeout string
)

var rootCmd = &cobra.Command{
	Use:   "supermetrics",
	Short: "Supermetrics CLI - interact with the Supermetrics API",
	Long: `Supermetrics CLI provides command-line access to the Supermetrics API.

Query marketing data, manage login links, schedule backfills, and more
from your terminal or scripts.

Get started:
  supermetrics configure             Set up your API key
  supermetrics login-links create    Create a new data source login link
  supermetrics queries execute       Run a data query`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Print update hint from previous background check
		update.PrintUpdateHint(infoWriterErr(), buildcfg.Version)

		// Spawn non-blocking background check if interval has elapsed
		if update.ShouldCheck() {
			update.NewUpdater().RunBackgroundCheck(buildcfg.Version)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key (overrides SUPERMETRICS_API_KEY and config file)")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "json", "Output format: json, table, csv")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&flagFlatten, "flatten", false, "Expand nested data in table output (CSV always flattens)")
	rootCmd.PersistentFlags().BoolVar(&flagNoRetry, "no-retry", false, "Disable automatic retry on transient errors")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress informational output")
	rootCmd.PersistentFlags().StringVar(&flagFields, "fields", "", "Comma-separated list of fields to include in output (e.g. id,status,error.message)")
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", "", "Named profile to use for credentials")
	rootCmd.PersistentFlags().StringVar(&flagTimeout, "timeout", "", "Override request timeout (e.g., 30s, 5m, 1h)")

	// Register flag value completions
	rootCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) { //nolint:errcheck // RegisterFlagCompletionFunc only fails if flag doesn't exist
		return []string{"json", "table", "csv"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Bind env vars via Viper
	viper.SetEnvPrefix("SUPERMETRICS")
	viper.AutomaticEnv()

	// Load .env file if present. Values only apply when no real env var
	// is set, giving us the precedence: real env var > .env > config file.
	// Combined with AutomaticEnv above and config loading below, the full
	// chain is: --flag > env var > .env > config file > hardcoded default.
	envViper := viper.New()
	envViper.SetConfigFile(".env")
	envViper.SetConfigType("env")
	if err := envViper.ReadInConfig(); err == nil {
		for _, key := range envViper.AllKeys() {
			if !viper.IsSet(key) {
				viper.Set(key, envViper.Get(key))
			}
		}
	}

	// Override flag defaults: config file first, then env vars.
	// Priority: --flag > env var > config file > hardcoded default.
	cfg, err := config.Load()
	if err == nil {
		if cfg.DefaultOutput != "" {
			flagOutput = cfg.DefaultOutput
			rootCmd.PersistentFlags().Lookup("output").DefValue = cfg.DefaultOutput
		}
	}
	// Register generated resource commands
	generated.RegisterAll(rootCmd)

	// Set up colored help output
	initHelpColorization(rootCmd)
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		var apiErr *httpclient.APIError
		if errors.As(err, &apiErr) {
			useColor := !flagNoColor && isStderrTerminal()
			if _, ok := os.LookupEnv("NO_COLOR"); ok {
				useColor = false
			}
			output.PrintError(os.Stderr, output.APIErrorFields{
				Message:     apiErr.Message,
				Description: apiErr.Description,
				Code:        apiErr.Code,
				RequestID:   apiErr.RequestID,
			}, flagOutput, useColor)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return exitcode.Wrap(err, classifyError(err))
	}
	return nil
}

// classifyError maps an error to a BSD sysexits-compatible exit code.
func classifyError(err error) int {
	// Already classified (e.g. from generated validation)
	var exitErr *exitcode.Error
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	// API errors — classify by HTTP status
	var apiErr *httpclient.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 401 || apiErr.StatusCode == 403:
			return exitcode.Auth
		case apiErr.StatusCode == 429 || apiErr.StatusCode >= 500:
			return exitcode.Unavailable
		}
		return 1
	}
	// Auth sentinel errors
	if errors.Is(err, auth.ErrNoCredentials) || errors.Is(err, auth.ErrTokenExpired) {
		return exitcode.Auth
	}
	// Network errors
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) {
		return exitcode.Unavailable
	}
	return 1
}

func isStderrTerminal() bool {
	return isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
}

// GetOutputFormat returns the current output format flag value.
func GetOutputFormat() string {
	return flagOutput
}

// GetAPIKeyFlag returns the --api-key flag value.
func GetAPIKeyFlag() string {
	return flagAPIKey
}

// GetProfile returns the resolved profile name.
// Priority: --profile flag > SUPERMETRICS_PROFILE env > active_profile in config > "default".
func GetProfile() string {
	if flagProfile != "" {
		return flagProfile
	}
	if val, ok := os.LookupEnv("SUPERMETRICS_PROFILE"); ok && val != "" {
		return val
	}
	cfg, err := config.Load()
	if err == nil && cfg.ActiveProfile != "" {
		return cfg.ActiveProfile
	}
	return config.DefaultProfile
}

// getDomain returns the API domain. Priority: SUPERMETRICS_DOMAIN env var > buildcfg default.
func getDomain() string {
	if d := viper.GetString("domain"); d != "" {
		return d
	}
	return buildcfg.DefaultDomain
}

// IsVerbose returns whether verbose mode is enabled.
func IsVerbose() bool {
	return flagVerbose
}

// isQuiet returns whether quiet mode is enabled via flag or env var.
func isQuiet() bool {
	if flagQuiet {
		return true
	}
	if val, ok := os.LookupEnv("SUPERMETRICS_QUIET"); ok && val != "" && val != "0" {
		return true
	}
	return false
}

// infoWriter returns a writer for informational stdout messages.
// Returns io.Discard in quiet mode.
func infoWriter() io.Writer {
	if isQuiet() {
		return io.Discard
	}
	return os.Stdout
}

// infoWriterErr returns a writer for informational stderr messages.
// Returns io.Discard in quiet mode.
func infoWriterErr() io.Writer {
	if isQuiet() {
		return io.Discard
	}
	return os.Stderr
}
