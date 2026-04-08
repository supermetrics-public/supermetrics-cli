package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/supermetrics-public/supermetrics-cli/internal/config"
)

func stubEnv(vals map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := vals[key]
		return v, ok
	}
}

// profileConfig creates a Config with credentials in the given profile.
func profileConfig(profileName string, profile *config.Profile) *config.Config {
	return &config.Config{
		Profiles: map[string]*config.Profile{
			profileName: profile,
		},
	}
}

func TestResolveToken_FlagTakesPriority(t *testing.T) {
	key, err := ResolveToken("flag-key", stubEnv(map[string]string{
		EnvAPIKey: "env-key",
	}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.NoError(t, err)
	assert.Equal(t, "flag-key", key)
}

func TestResolveToken_EnvFallback(t *testing.T) {
	key, err := ResolveToken("", stubEnv(map[string]string{
		EnvAPIKey: "env-key",
	}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.NoError(t, err)
	assert.Equal(t, "env-key", key)
}

func TestResolveToken_NoKeyError(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	_, err := ResolveToken("", stubEnv(map[string]string{}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.Error(t, err)
	assert.True(t, assert.Contains(t, err.Error(), "supermetrics login") || assert.Contains(t, err.Error(), "supermetrics configure"), "error should suggest login or configure")
}

func TestResolveToken_OAuthTokenUsed(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := profileConfig(config.DefaultProfile, &config.Profile{
		AccessToken: "oauth-token",
		TokenExpiry: "2099-01-01T00:00:00Z",
	})
	require.NoError(t, config.Save(cfg))

	key, err := ResolveToken("", stubEnv(map[string]string{}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.NoError(t, err)
	assert.Equal(t, "oauth-token", key)
}

func TestResolveToken_OAuthTakesPrecedenceOverAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := profileConfig(config.DefaultProfile, &config.Profile{
		APIKey:      "api-key-123",
		AccessToken: "oauth-token",
		TokenExpiry: "2099-01-01T00:00:00Z",
	})
	require.NoError(t, config.Save(cfg))

	key, err := ResolveToken("", stubEnv(map[string]string{}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.NoError(t, err)
	assert.Equal(t, "oauth-token", key)
}

func TestResolveToken_FlagOverridesOAuth(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := profileConfig(config.DefaultProfile, &config.Profile{
		AccessToken: "oauth-token",
		TokenExpiry: "2099-01-01T00:00:00Z",
	})
	require.NoError(t, config.Save(cfg))

	key, err := ResolveToken("explicit-key", stubEnv(map[string]string{}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.NoError(t, err)
	assert.Equal(t, "explicit-key", key)
}

func TestResolveToken_ExpiredTokenWithoutRefreshErrors(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := profileConfig(config.DefaultProfile, &config.Profile{
		AccessToken: "expired-token",
		TokenExpiry: "2020-01-01T00:00:00Z",
	})
	require.NoError(t, config.Save(cfg))

	_, err := ResolveToken("", stubEnv(map[string]string{}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expired")
}

func TestResolveToken_FallsBackToAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := profileConfig(config.DefaultProfile, &config.Profile{APIKey: "fallback-key"})
	require.NoError(t, config.Save(cfg))

	key, err := ResolveToken("", stubEnv(map[string]string{}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.NoError(t, err)
	assert.Equal(t, "fallback-key", key)
}

func TestResolveToken_EmptyEnvIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	// Empty string env var should be ignored
	_, err := ResolveToken("", stubEnv(map[string]string{
		EnvAPIKey: "",
	}), "supermetrics.com", testOAuthCfg, config.DefaultProfile)
	require.Error(t, err)

	// Verify config dir was created properly
	dir, _ := config.Dir()
	assert.True(t, len(dir) > 0 && dir[:len(tmpDir)] == tmpDir, "config dir should be under tmpDir")
	_ = filepath.Join(dir) // suppress unused
}

func TestResolveToken_ExpiredTokenRefreshSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := profileConfig(config.DefaultProfile, &config.Profile{
		AccessToken:  "expired-token",
		RefreshToken: "my-refresh-token",
		TokenExpiry:  "2020-01-01T00:00:00Z",
	})
	require.NoError(t, config.Save(cfg))

	// Mock OAuth server returns new tokens
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.FormValue("grant_type"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "refreshed-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer cleanup()

	key, err := ResolveToken("", stubEnv(map[string]string{}), "example.com", testOAuthCfg, config.DefaultProfile)
	require.NoError(t, err)
	assert.Equal(t, "refreshed-access-token", key)

	// Verify config was updated
	saved, err := config.Load()
	require.NoError(t, err)
	p := saved.GetProfile(config.DefaultProfile)
	assert.Equal(t, "refreshed-access-token", p.AccessToken)
	assert.Equal(t, "new-refresh-token", p.RefreshToken)
	assert.NotEmpty(t, p.TokenExpiry)
	assert.NotEqual(t, "2020-01-01T00:00:00Z", p.TokenExpiry)
}

func TestResolveToken_ExpiredTokenRefreshFails(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := profileConfig(config.DefaultProfile, &config.Profile{
		AccessToken:  "expired-token",
		RefreshToken: "bad-refresh-token",
		TokenExpiry:  "2020-01-01T00:00:00Z",
	})
	require.NoError(t, config.Save(cfg))

	// Mock OAuth server returns error
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "The refresh token is expired",
		})
	}))
	defer cleanup()

	_, err := ResolveToken("", stubEnv(map[string]string{}), "example.com", testOAuthCfg, config.DefaultProfile)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expired")

	// Verify tokens were cleared
	saved, err := config.Load()
	require.NoError(t, err)
	p := saved.GetProfile(config.DefaultProfile)
	assert.Empty(t, p.AccessToken)
	assert.Empty(t, p.RefreshToken)
}

func TestResolveToken_RefreshKeepsExistingRefreshToken(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := profileConfig(config.DefaultProfile, &config.Profile{
		AccessToken:  "expired-token",
		RefreshToken: "original-refresh",
		TokenExpiry:  "2020-01-01T00:00:00Z",
	})
	require.NoError(t, config.Save(cfg))

	// Server returns empty refresh_token — should keep original
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-access",
			"expires_in":   3600,
			"token_type":   "Bearer",
		})
	}))
	defer cleanup()

	key, err := ResolveToken("", stubEnv(map[string]string{}), "example.com", testOAuthCfg, config.DefaultProfile)
	require.NoError(t, err)
	assert.Equal(t, "new-access", key)

	// Original refresh token should be preserved
	saved, err := config.Load()
	require.NoError(t, err)
	p := saved.GetProfile(config.DefaultProfile)
	assert.Equal(t, "original-refresh", p.RefreshToken)
}

func TestResolveToken_NamedProfile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origHome)

	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"default": {APIKey: "default-key"},
			"work":    {APIKey: "work-key"},
		},
	}
	require.NoError(t, config.Save(cfg))

	key, err := ResolveToken("", stubEnv(map[string]string{}), "supermetrics.com", testOAuthCfg, "work")
	require.NoError(t, err)
	assert.Equal(t, "work-key", key)
}
