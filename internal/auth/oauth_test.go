package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/supermetrics-public/supermetrics-cli/internal/buildcfg"
)

var testOAuthCfg = OAuthConfig{ClientID: "test-client-id", Scopes: "openid"}

// testTransport redirects requests from https://api.{domain}/... to a local test server.
type testTransport struct {
	server *httptest.Server
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to our test server
	u, _ := url.Parse(t.server.URL)
	req.URL.Scheme = u.Scheme
	req.URL.Host = u.Host
	return http.DefaultTransport.RoundTrip(req)
}

func withTestServer(handler http.Handler) (*httptest.Server, func()) {
	server := httptest.NewServer(handler)
	original := oauthHTTPClient
	oauthHTTPClient = &http.Client{Transport: &testTransport{server: server}}
	cleanup := func() {
		oauthHTTPClient = original
		server.Close()
	}
	return server, cleanup
}

func TestPostTokenRequest_Success(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access",
			"refresh_token": "test-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer cleanup()

	token, err := postTokenRequest(context.Background(), "example.com", url.Values{
		"grant_type": {"test"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test-access", token.AccessToken)
	assert.Equal(t, "test-refresh", token.RefreshToken)
	assert.Equal(t, 3600, token.ExpiresIn)
	assert.Equal(t, "Bearer", token.TokenType)
}

func TestPostTokenRequest_ErrorWithDescription(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "The refresh token is expired",
		})
	}))
	defer cleanup()

	_, err := postTokenRequest(context.Background(), "example.com", url.Values{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "The refresh token is expired")
}

func TestPostTokenRequest_ErrorWithoutDescription(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer cleanup()

	_, err := postTokenRequest(context.Background(), "example.com", url.Values{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "500")
}

func TestPostTokenRequest_InvalidJSON(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json at all")
	}))
	defer cleanup()

	_, err := postTokenRequest(context.Background(), "example.com", url.Values{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "parse")
}

func TestRefresh_Success(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.FormValue("grant_type"))
		assert.Equal(t, "my-refresh", r.FormValue("refresh_token"))
		assert.Equal(t, testOAuthCfg.ClientID, r.FormValue("client_id"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    7200,
			"token_type":    "Bearer",
		})
	}))
	defer cleanup()

	token, err := Refresh(context.Background(), "example.com", "my-refresh", testOAuthCfg)
	require.NoError(t, err)
	assert.Equal(t, "new-access", token.AccessToken)
	assert.Equal(t, "new-refresh", token.RefreshToken)
}

func TestRevoke_Success(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "revoke-me", r.FormValue("token"))
		w.WriteHeader(200)
	}))
	defer cleanup()

	err := Revoke(context.Background(), "example.com", "revoke-me", testOAuthCfg)
	assert.NoError(t, err)
}

func TestRevoke_ServerError(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer cleanup()

	err := Revoke(context.Background(), "example.com", "token", testOAuthCfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "500")
}

func TestGeneratePKCE(t *testing.T) {
	verifier, challenge, err := generatePKCE()
	require.NoError(t, err)

	// Verifier should be base64url-encoded 64 bytes
	decoded, err := base64.RawURLEncoding.DecodeString(verifier)
	require.NoError(t, err)
	assert.Equal(t, 64, len(decoded))

	// Challenge should be S256 of verifier
	h := sha256.Sum256([]byte(verifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	assert.Equal(t, expectedChallenge, challenge)

	// Two calls should produce different values
	verifier2, _, err := generatePKCE()
	require.NoError(t, err)
	assert.NotEqual(t, verifier, verifier2)
}

func TestLoadOAuthConfig_FromViper(t *testing.T) {
	ResetOAuthConfigCache()
	viper.Set("oauth_client_id", "viper-client")
	viper.Set("oauth_scopes", "viper-scopes")
	defer viper.Reset()

	cfg, err := LoadOAuthConfig()
	require.NoError(t, err)
	assert.Equal(t, "viper-client", cfg.ClientID)
	assert.Equal(t, "viper-scopes", cfg.Scopes)
}

func TestLoadOAuthConfig_FallbackToBuildCfg(t *testing.T) {
	ResetOAuthConfigCache()
	viper.Reset()

	origID := buildcfg.OAuthClientID
	origScopes := buildcfg.OAuthScopes
	defer func() {
		buildcfg.OAuthClientID = origID
		buildcfg.OAuthScopes = origScopes
	}()

	buildcfg.OAuthClientID = "build-client"
	buildcfg.OAuthScopes = "build-scopes"

	cfg, err := LoadOAuthConfig()
	require.NoError(t, err)
	assert.Equal(t, "build-client", cfg.ClientID)
	assert.Equal(t, "build-scopes", cfg.Scopes)
}

func TestLoadOAuthConfig_BothEmpty(t *testing.T) {
	ResetOAuthConfigCache()
	viper.Reset()

	origID := buildcfg.OAuthClientID
	origScopes := buildcfg.OAuthScopes
	defer func() {
		buildcfg.OAuthClientID = origID
		buildcfg.OAuthScopes = origScopes
	}()

	buildcfg.OAuthClientID = ""
	buildcfg.OAuthScopes = ""

	_, err := LoadOAuthConfig()
	require.Error(t, err)
	assert.ErrorContains(t, err, "SUPERMETRICS_OAUTH_CLIENT_ID")
}

func TestLoadOAuthConfig_ClientIDSetScopesEmpty(t *testing.T) {
	ResetOAuthConfigCache()
	viper.Set("oauth_client_id", "some-client")
	defer viper.Reset()

	origScopes := buildcfg.OAuthScopes
	defer func() { buildcfg.OAuthScopes = origScopes }()
	buildcfg.OAuthScopes = ""

	_, err := LoadOAuthConfig()
	require.Error(t, err)
	assert.ErrorContains(t, err, "SUPERMETRICS_OAUTH_SCOPES")
}

func TestLoadOAuthConfig_ViperOverridesBuildCfg(t *testing.T) {
	ResetOAuthConfigCache()
	origID := buildcfg.OAuthClientID
	origScopes := buildcfg.OAuthScopes
	defer func() {
		buildcfg.OAuthClientID = origID
		buildcfg.OAuthScopes = origScopes
	}()

	buildcfg.OAuthClientID = "build-client"
	buildcfg.OAuthScopes = "build-scopes"

	viper.Set("oauth_client_id", "viper-client")
	viper.Set("oauth_scopes", "viper-scopes")
	defer viper.Reset()

	cfg, err := LoadOAuthConfig()
	require.NoError(t, err)
	assert.Equal(t, "viper-client", cfg.ClientID)
	assert.Equal(t, "viper-scopes", cfg.Scopes)
}

func TestLogin_Timeout(t *testing.T) {
	origOpen := openBrowser
	openBrowser = func(string) error { return nil }
	defer func() { openBrowser = origOpen }()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := Login(ctx, "example.com", testOAuthCfg, io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "timed out")
}

func TestExchangeCode_Success(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.FormValue("grant_type"))
		assert.Equal(t, "auth-code-123", r.FormValue("code"))
		assert.Equal(t, "http://localhost:9999/callback", r.FormValue("redirect_uri"))
		assert.Equal(t, "test-verifier", r.FormValue("code_verifier"))
		assert.Equal(t, testOAuthCfg.ClientID, r.FormValue("client_id"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "exchanged-token",
			"refresh_token": "exchanged-refresh",
			"expires_in":    7200,
			"token_type":    "Bearer",
		})
	}))
	defer cleanup()

	token, err := exchangeCode(context.Background(), "example.com", "auth-code-123", "http://localhost:9999/callback", "test-verifier", testOAuthCfg)
	require.NoError(t, err)
	assert.Equal(t, "exchanged-token", token.AccessToken)
	assert.Equal(t, 7200, token.ExpiresIn)
}

func TestGenerateState(t *testing.T) {
	state, err := generateState()
	require.NoError(t, err)

	decoded, err := base64.RawURLEncoding.DecodeString(state)
	require.NoError(t, err)
	assert.Equal(t, 32, len(decoded))

	state2, err := generateState()
	require.NoError(t, err)
	assert.NotEqual(t, state, state2)
}

func TestLogin_StateInAuthorizeURL(t *testing.T) {
	origOpen := openBrowser
	var capturedURL string
	openBrowser = func(u string) error {
		capturedURL = u
		return nil
	}
	defer func() { openBrowser = origOpen }()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Login will time out, but we can still inspect the URL
	_, _ = Login(ctx, "example.com", testOAuthCfg, io.Discard)

	parsed, err := url.Parse(capturedURL)
	require.NoError(t, err)
	state := parsed.Query().Get("state")
	assert.NotEmpty(t, state)
	// Verify it's valid base64url
	decoded, err := base64.RawURLEncoding.DecodeString(state)
	require.NoError(t, err)
	assert.Equal(t, 32, len(decoded))
}

func TestLogin_StateMismatchRejected(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access",
			"refresh_token": "test-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer cleanup()

	// Use a separate client to hit the callback (http.DefaultClient transport is overridden by withTestServer)
	directClient := &http.Client{}

	origOpen := openBrowser
	openBrowser = func(u string) error {
		parsed, _ := url.Parse(u)
		redirectURI := parsed.Query().Get("redirect_uri")
		callbackURL := redirectURI + "?code=auth-code&state=wrong-state"
		go func() {
			time.Sleep(50 * time.Millisecond)
			resp, err := directClient.Get(callbackURL)
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	defer func() { openBrowser = origOpen }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Login(ctx, "example.com", testOAuthCfg, io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "state parameter mismatch")
}

func TestLogin_CorrectStateAccepted(t *testing.T) {
	_, cleanup := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access",
			"refresh_token": "test-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer cleanup()

	directClient := &http.Client{}

	origOpen := openBrowser
	openBrowser = func(u string) error {
		parsed, _ := url.Parse(u)
		redirectURI := parsed.Query().Get("redirect_uri")
		state := parsed.Query().Get("state")
		callbackURL := fmt.Sprintf("%s?code=auth-code&state=%s", redirectURI, url.QueryEscape(state))
		go func() {
			time.Sleep(50 * time.Millisecond)
			resp, err := directClient.Get(callbackURL)
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	defer func() { openBrowser = origOpen }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	token, err := Login(ctx, "example.com", testOAuthCfg, io.Discard)
	require.NoError(t, err)
	assert.Equal(t, "test-access", token.AccessToken)
}

func TestLogin_CallbackErrorDescription(t *testing.T) {
	directClient := &http.Client{}

	origOpen := openBrowser
	openBrowser = func(u string) error {
		parsed, _ := url.Parse(u)
		redirectURI := parsed.Query().Get("redirect_uri")
		state := parsed.Query().Get("state")
		callbackURL := fmt.Sprintf("%s?state=%s&error_description=access_denied", redirectURI, url.QueryEscape(state))
		go func() {
			time.Sleep(50 * time.Millisecond)
			resp, err := directClient.Get(callbackURL)
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	defer func() { openBrowser = origOpen }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Login(ctx, "example.com", testOAuthCfg, io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "access_denied")
}

func TestLogin_CallbackErrorParam(t *testing.T) {
	directClient := &http.Client{}

	origOpen := openBrowser
	openBrowser = func(u string) error {
		parsed, _ := url.Parse(u)
		redirectURI := parsed.Query().Get("redirect_uri")
		state := parsed.Query().Get("state")
		callbackURL := fmt.Sprintf("%s?state=%s&error=invalid_request", redirectURI, url.QueryEscape(state))
		go func() {
			time.Sleep(50 * time.Millisecond)
			resp, err := directClient.Get(callbackURL)
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	defer func() { openBrowser = origOpen }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Login(ctx, "example.com", testOAuthCfg, io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid_request")
}

func TestLogin_CallbackNoCodeNoError(t *testing.T) {
	directClient := &http.Client{}

	origOpen := openBrowser
	openBrowser = func(u string) error {
		parsed, _ := url.Parse(u)
		redirectURI := parsed.Query().Get("redirect_uri")
		state := parsed.Query().Get("state")
		// No code, no error, no error_description — just state
		callbackURL := fmt.Sprintf("%s?state=%s", redirectURI, url.QueryEscape(state))
		go func() {
			time.Sleep(50 * time.Millisecond)
			resp, err := directClient.Get(callbackURL)
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	defer func() { openBrowser = origOpen }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Login(ctx, "example.com", testOAuthCfg, io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no authorization code received")
}

func TestLogin_BrowserOpenError(t *testing.T) {
	origOpen := openBrowser
	openBrowser = func(string) error {
		return fmt.Errorf("no display available")
	}
	defer func() { openBrowser = origOpen }()

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Login will time out since no callback arrives, but the fallback URL message should be printed
	_, _ = Login(ctx, "example.com", testOAuthCfg, &buf)

	output := buf.String()
	assert.Contains(t, output, "Could not open browser automatically")
	assert.Contains(t, output, "https://api.example.com/oauth/authorize")
}

func TestHtmlPage(t *testing.T) {
	page := htmlPage("Test Title", "Test message body")
	assert.Contains(t, page, "Test Title")
	assert.Contains(t, page, "Test message body")
	assert.Contains(t, page, "<!DOCTYPE html>")
}
