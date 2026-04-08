package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	configDir  = "supermetrics"
	configFile = "config.json"

	// DefaultProfile is the name used when no profile is specified.
	DefaultProfile = "default"
)

// Config represents the CLI configuration stored on disk.
type Config struct {
	// Global settings (not per-profile)
	DefaultOutput           string `json:"default_output,omitempty"`
	UpdateCheckIntervalDays int    `json:"update_check_interval_days,omitempty"`
	LastUpdateCheck         string `json:"last_update_check,omitempty"`
	AvailableUpdate         string `json:"available_update,omitempty"`

	// Profile management
	ActiveProfile string              `json:"active_profile,omitempty"`
	Profiles      map[string]*Profile `json:"profiles,omitempty"`
}

// Profile holds credentials for a named profile.
type Profile struct {
	APIKey       string `json:"api_key,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenExpiry  string `json:"token_expiry,omitempty"` // RFC3339
}

// Dir returns the configuration directory path (~/.config/supermetrics).
func Dir() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configHome, configDir), nil
}

// Path returns the full path to the config file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFile), nil
}

// Load reads the config file from disk. Returns a zero-value Config if the file doesn't exist.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return &Config{}, nil
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes the config to disk, creating the directory if needed.
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	p := filepath.Join(dir, configFile)
	return os.WriteFile(p, data, 0o600)
}

// ActiveOrDefault returns the active profile name, falling back to DefaultProfile.
func (c *Config) ActiveOrDefault() string {
	if c.ActiveProfile != "" {
		return c.ActiveProfile
	}
	return DefaultProfile
}

// GetProfile returns the profile with the given name, creating it if it doesn't exist.
func (c *Config) GetProfile(name string) *Profile {
	if c.Profiles == nil {
		c.Profiles = make(map[string]*Profile)
	}
	if p, ok := c.Profiles[name]; ok {
		return p
	}
	p := &Profile{}
	c.Profiles[name] = p
	return p
}

// Validate checks that config values are valid.
func (c *Config) Validate() error {
	if c.DefaultOutput != "" {
		switch c.DefaultOutput {
		case "json", "table", "csv":
			// valid
		default:
			return fmt.Errorf("invalid default output format %q: must be json, table, or csv", c.DefaultOutput)
		}
	}

	for name, p := range c.Profiles {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
	}

	if c.UpdateCheckIntervalDays < 0 {
		return fmt.Errorf("update check interval must be non-negative, got %d", c.UpdateCheckIntervalDays)
	}

	return nil
}

// Validate checks that profile values are valid.
func (p *Profile) Validate() error {
	if p.APIKey != "" {
		if !strings.HasPrefix(p.APIKey, "api_") {
			return fmt.Errorf("invalid API key: must start with \"api_\"")
		}
		if len(p.APIKey) < 60 {
			return fmt.Errorf("invalid API key: too short (expected at least 60 characters, got %d)", len(p.APIKey))
		}
	}
	return nil
}

// IsTokenExpired returns true if the stored OAuth token has expired or is about to expire.
func (p *Profile) IsTokenExpired() bool {
	if p.AccessToken == "" || p.TokenExpiry == "" {
		return true
	}
	expiry, err := time.Parse(time.RFC3339, p.TokenExpiry)
	if err != nil {
		return true
	}
	// Consider expired 60 seconds early to avoid race conditions
	return time.Now().After(expiry.Add(-60 * time.Second))
}

// ClearOAuthTokens removes all OAuth token fields from the profile.
func (p *Profile) ClearOAuthTokens() {
	p.AccessToken = ""
	p.RefreshToken = ""
	p.TokenExpiry = ""
}

// UpdateCheckInterval returns the configured interval, defaulting to 7 days.
func (c *Config) UpdateCheckInterval() time.Duration {
	days := c.UpdateCheckIntervalDays
	if days <= 0 {
		days = 7
	}
	return time.Duration(days) * 24 * time.Hour
}
