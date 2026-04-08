package auth

import (
	"context"
	"errors"
	"time"

	"github.com/supermetrics-public/supermetrics-cli/internal/config"
)

// Sentinel errors for credential resolution failures.
var (
	ErrNoCredentials = errors.New("no credentials found. Run 'supermetrics login' or 'supermetrics configure'")
	ErrTokenExpired  = errors.New("OAuth token expired. Run 'supermetrics login' to re-authenticate")
)

const EnvAPIKey = "SUPERMETRICS_API_KEY"

// ResolveToken returns a Bearer token using the priority:
// flag > env > OAuth token (auto-refresh if expired) > API key from config.
// The profileName selects which config profile to read credentials from.
// Returns an error with guidance if no credentials are found.
func ResolveToken(flagValue string, envLookup func(string) (string, bool), domain string, oauthCfg OAuthConfig, profileName string) (string, error) {
	// 1. CLI flag (highest priority)
	if flagValue != "" {
		return flagValue, nil
	}

	// 2. Environment variable
	if val, ok := envLookup(EnvAPIKey); ok && val != "" {
		return val, nil
	}

	// 3. Config file
	cfg, err := config.Load()
	if err != nil {
		return "", ErrNoCredentials
	}

	profile := cfg.GetProfile(profileName)

	// 3a. OAuth token (takes precedence over API key)
	if profile.AccessToken != "" {
		if !profile.IsTokenExpired() {
			return profile.AccessToken, nil
		}

		// Token expired — try to refresh
		if profile.RefreshToken != "" {
			token, err := Refresh(context.Background(), domain, profile.RefreshToken, oauthCfg)
			if err == nil {
				profile.AccessToken = token.AccessToken
				if token.RefreshToken != "" {
					profile.RefreshToken = token.RefreshToken
				}
				profile.TokenExpiry = token.Expiry().Format(time.RFC3339)
				_ = config.Save(cfg) // Best effort
				return profile.AccessToken, nil
			}
			// Refresh failed — clear tokens and fall through
			profile.ClearOAuthTokens()
			_ = config.Save(cfg)
		}

		return "", ErrTokenExpired
	}

	// 3b. API key
	if profile.APIKey != "" {
		return profile.APIKey, nil
	}

	return "", ErrNoCredentials
}
