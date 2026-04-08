package cmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/supermetrics-public/supermetrics-cli/internal/auth"
	"github.com/supermetrics-public/supermetrics-cli/internal/config"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in with your Supermetrics account",
	Long:  `Authenticate via OAuth using your Google or Microsoft account. Opens a browser for login.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showStatus, _ := cmd.Flags().GetBool("status")
		if showStatus {
			return printLoginStatus(infoWriter())
		}

		domain := getDomain()

		oauthCfg, err := auth.LoadOAuthConfig()
		if err != nil {
			return err
		}

		token, err := auth.Login(context.Background(), domain, oauthCfg, infoWriter())
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			cfg = &config.Config{}
		}

		profileName := GetProfile()
		profile := cfg.GetProfile(profileName)

		profile.AccessToken = token.AccessToken
		profile.RefreshToken = token.RefreshToken
		profile.TokenExpiry = token.Expiry().Format(time.RFC3339)

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		fmt.Fprintf(infoWriter(), "Logged in successfully. Token expires in %s and will be refreshed automatically.\n", formatDuration(time.Until(token.Expiry())))
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and revoke stored OAuth tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := infoWriter()

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(w, "Not logged in.")
			return nil
		}

		profileName := GetProfile()
		profile := cfg.GetProfile(profileName)

		if profile.AccessToken == "" {
			fmt.Fprintln(w, "Not logged in.")
			return nil
		}

		domain := getDomain()

		// Best-effort revocation — clear tokens even if revoke fails
		oauthCfg, _ := auth.LoadOAuthConfig()
		revokeErr := auth.Revoke(context.Background(), domain, profile.AccessToken, oauthCfg)
		if revokeErr != nil {
			fmt.Fprintf(infoWriterErr(), "Warning: failed to revoke token: %v\n", revokeErr)
		}

		profile.ClearOAuthTokens()
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if revokeErr != nil {
			fmt.Fprintln(w, "Logged out locally. Token may still be active server-side until it expires.")
		} else {
			fmt.Fprintln(w, "Logged out successfully.")
		}
		return nil
	},
}

func printLoginStatus(w io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(w, "Not logged in. Run 'supermetrics login' to authenticate.")
		return nil
	}

	profileName := GetProfile()
	profile := cfg.GetProfile(profileName)

	if profile.AccessToken == "" {
		fmt.Fprintln(w, "Not logged in. Run 'supermetrics login' to authenticate.")
		if profile.APIKey != "" {
			fmt.Fprintln(w, "Using API key for authentication.")
		}
		return nil
	}

	if profile.IsTokenExpired() {
		if profile.RefreshToken != "" {
			fmt.Fprintln(w, "OAuth token expired (will auto-refresh on next command).")
		} else {
			fmt.Fprintln(w, "OAuth token expired. Run 'supermetrics login' to re-authenticate.")
		}
		return nil
	}

	expiry, _ := time.Parse(time.RFC3339, profile.TokenExpiry)
	remaining := time.Until(expiry)
	if remaining < time.Minute {
		fmt.Fprintln(w, "Logged in via OAuth. Token expires in less than a minute (will auto-refresh on next command).")
	} else {
		fmt.Fprintf(w, "Logged in via OAuth. Token expires in %s and will be refreshed automatically.\n", formatDuration(remaining))
	}
	return nil
}

func formatDuration(d time.Duration) string {
	minutes := int(d.Round(time.Minute).Minutes())
	if minutes == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", minutes)
}

func init() {
	loginCmd.Flags().Bool("status", false, "Show current authentication status")
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}
