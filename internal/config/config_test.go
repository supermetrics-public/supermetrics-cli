package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTempConfig(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	orig := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	return func() { os.Setenv("XDG_CONFIG_HOME", orig) }
}

func TestLoadEmpty(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.Profiles)
}

func TestSaveAndLoad(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	original := &Config{
		DefaultOutput:           "table",
		UpdateCheckIntervalDays: 14,
		Profiles: map[string]*Profile{
			"default": {APIKey: "api_" + string(make([]byte, 56))},
		},
	}

	require.NoError(t, Save(original))

	loaded, err := Load()
	require.NoError(t, err)

	assert.Equal(t, original.DefaultOutput, loaded.DefaultOutput)
	assert.Equal(t, original.UpdateCheckIntervalDays, loaded.UpdateCheckIntervalDays)
	assert.Equal(t, original.Profiles["default"].APIKey, loaded.Profiles["default"].APIKey)
}

func TestConfigFilePermissions(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	require.NoError(t, Save(&Config{
		Profiles: map[string]*Profile{"default": {APIKey: "secret"}},
	}))

	p, _ := Path()
	info, err := os.Stat(p)
	require.NoError(t, err)

	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0o600), perm)
}

func TestConfigDirPermissions(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	require.NoError(t, Save(&Config{}))

	dir, _ := Dir()
	info, err := os.Stat(dir)
	require.NoError(t, err)

	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0o700), perm)
}

func TestUpdateCheckInterval_Default(t *testing.T) {
	cfg := &Config{}
	interval := cfg.UpdateCheckInterval()
	assert.Equal(t, 7*24*time.Hour, interval)
}

func TestUpdateCheckInterval_Custom(t *testing.T) {
	cfg := &Config{UpdateCheckIntervalDays: 3}
	interval := cfg.UpdateCheckInterval()
	assert.Equal(t, 3*24*time.Hour, interval)
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		DefaultOutput: "json",
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_EmptyIsValid(t *testing.T) {
	cfg := &Config{}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_InvalidOutputFormat(t *testing.T) {
	cfg := &Config{DefaultOutput: "xml"}
	assert.Error(t, cfg.Validate())
}

func TestValidate_NegativeInterval(t *testing.T) {
	cfg := &Config{UpdateCheckIntervalDays: -1}
	assert.Error(t, cfg.Validate())
}

func TestValidate_ValidAPIKey(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]*Profile{
			"default": {APIKey: "api_" + string(make([]byte, 56))},
		},
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_EmptyAPIKeyIsValid(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]*Profile{
			"default": {APIKey: ""},
		},
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_APIKeyWrongPrefix(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]*Profile{
			"default": {APIKey: "sk_" + string(make([]byte, 57))},
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "must start with")
}

func TestValidate_APIKeyTooShort(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]*Profile{
			"default": {APIKey: "api_short"},
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "too short")
}

func TestIsTokenExpired_ValidToken(t *testing.T) {
	p := &Profile{
		AccessToken: "token",
		TokenExpiry: time.Now().Add(10 * time.Minute).Format(time.RFC3339),
	}
	assert.False(t, p.IsTokenExpired())
}

func TestIsTokenExpired_ExpiredToken(t *testing.T) {
	p := &Profile{
		AccessToken: "token",
		TokenExpiry: time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
	}
	assert.True(t, p.IsTokenExpired())
}

func TestIsTokenExpired_EmptyAccessToken(t *testing.T) {
	p := &Profile{
		AccessToken: "",
		TokenExpiry: time.Now().Add(10 * time.Minute).Format(time.RFC3339),
	}
	assert.True(t, p.IsTokenExpired())
}

func TestIsTokenExpired_EmptyExpiry(t *testing.T) {
	p := &Profile{
		AccessToken: "token",
		TokenExpiry: "",
	}
	assert.True(t, p.IsTokenExpired())
}

func TestIsTokenExpired_InvalidExpiry(t *testing.T) {
	p := &Profile{
		AccessToken: "token",
		TokenExpiry: "not-a-date",
	}
	assert.True(t, p.IsTokenExpired())
}

func TestIsTokenExpired_ExpiresWithin60Seconds(t *testing.T) {
	p := &Profile{
		AccessToken: "token",
		TokenExpiry: time.Now().Add(30 * time.Second).Format(time.RFC3339),
	}
	assert.True(t, p.IsTokenExpired())
}

func TestClearOAuthTokens(t *testing.T) {
	p := &Profile{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenExpiry:  "2099-01-01T00:00:00Z",
		APIKey:       "should-remain",
	}
	p.ClearOAuthTokens()

	assert.Equal(t, "", p.AccessToken)
	assert.Equal(t, "", p.RefreshToken)
	assert.Equal(t, "", p.TokenExpiry)
	assert.Equal(t, "should-remain", p.APIKey)
}

func TestLoadMalformedJSON(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	// Save valid config first to create the directory
	require.NoError(t, Save(&Config{}))

	// Overwrite with malformed JSON
	p, _ := Path()
	require.NoError(t, os.WriteFile(p, []byte("{invalid json"), 0o600))

	_, err := Load()
	assert.Error(t, err)
}

func TestPath(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	p, err := Path()
	require.NoError(t, err)

	assert.Equal(t, "config.json", filepath.Base(p))
	assert.True(t, filepath.IsAbs(p))
}

func TestActiveOrDefault_Empty(t *testing.T) {
	cfg := &Config{}
	assert.Equal(t, DefaultProfile, cfg.ActiveOrDefault())
}

func TestActiveOrDefault_Set(t *testing.T) {
	cfg := &Config{ActiveProfile: "work"}
	assert.Equal(t, "work", cfg.ActiveOrDefault())
}

func TestGetProfile_CreatesNew(t *testing.T) {
	cfg := &Config{}
	p := cfg.GetProfile("work")
	assert.NotNil(t, p)
	assert.Equal(t, p, cfg.Profiles["work"])
}

func TestGetProfile_ReturnsExisting(t *testing.T) {
	existing := &Profile{APIKey: "api_" + string(make([]byte, 56))}
	cfg := &Config{
		Profiles: map[string]*Profile{"work": existing},
	}
	p := cfg.GetProfile("work")
	assert.Same(t, existing, p)
}

func TestGetProfile_InitializesMap(t *testing.T) {
	cfg := &Config{}
	assert.Nil(t, cfg.Profiles)
	cfg.GetProfile("default")
	assert.NotNil(t, cfg.Profiles)
}

func TestValidate_MultipleProfiles(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]*Profile{
			"valid":   {APIKey: "api_" + string(make([]byte, 56))},
			"invalid": {APIKey: "bad_key"},
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid")
}

func TestSaveAndLoadMultipleProfiles(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	original := &Config{
		ActiveProfile: "work",
		Profiles: map[string]*Profile{
			"default": {APIKey: "api_" + string(make([]byte, 56))},
			"work":    {AccessToken: "token123", TokenExpiry: "2099-01-01T00:00:00Z"},
		},
	}

	require.NoError(t, Save(original))

	loaded, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "work", loaded.ActiveProfile)
	assert.Len(t, loaded.Profiles, 2)
	assert.Equal(t, "token123", loaded.Profiles["work"].AccessToken)
}
