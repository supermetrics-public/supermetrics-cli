package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/supermetrics-public/supermetrics-cli/internal/buildcfg"
)

const (
	callbackPath   = "/callback"
	loginTimeout   = 5 * time.Minute
	refreshTimeout = 30 * time.Second
)

// oauthHTTPClient is the HTTP client used for OAuth token operations.
// Defaults to http.DefaultClient. Tests can override for injection.
var oauthHTTPClient *http.Client

func getOAuthHTTPClient() *http.Client {
	if oauthHTTPClient != nil {
		return oauthHTTPClient
	}
	return http.DefaultClient
}

// OAuthConfig holds OAuth settings loaded from environment.
type OAuthConfig struct {
	ClientID string
	Scopes   string
}

var (
	oauthConfigOnce   sync.Once
	cachedOAuthConfig OAuthConfig
	cachedOAuthErr    error
)

// LoadOAuthConfig reads OAuth settings from viper (SUPERMETRICS_OAUTH_CLIENT_ID,
// SUPERMETRICS_OAUTH_SCOPES). The result is cached after the first call.
func LoadOAuthConfig() (OAuthConfig, error) {
	oauthConfigOnce.Do(func() {
		clientID := viper.GetString("oauth_client_id")
		if clientID == "" {
			clientID = buildcfg.OAuthClientID
		}
		scopes := viper.GetString("oauth_scopes")
		if scopes == "" {
			scopes = buildcfg.OAuthScopes
		}
		if clientID == "" {
			cachedOAuthErr = fmt.Errorf("SUPERMETRICS_OAUTH_CLIENT_ID is not set")
			return
		}
		if scopes == "" {
			cachedOAuthErr = fmt.Errorf("SUPERMETRICS_OAUTH_SCOPES is not set")
			return
		}
		cachedOAuthConfig = OAuthConfig{ClientID: clientID, Scopes: scopes}
	})
	return cachedOAuthConfig, cachedOAuthErr
}

// ResetOAuthConfigCache resets the cached OAuth config so LoadOAuthConfig
// re-reads values on next call. Only useful in tests.
func ResetOAuthConfigCache() {
	oauthConfigOnce = sync.Once{}
	cachedOAuthConfig = OAuthConfig{}
	cachedOAuthErr = nil
}

// OAuthToken holds the tokens returned by the OAuth server.
type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	TokenType    string
}

// Expiry returns the absolute expiry time based on ExpiresIn.
func (t *OAuthToken) Expiry() time.Time {
	return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
}

// Login performs the Authorization Code + PKCE flow:
// 1. Starts a localhost HTTP server
// 2. Opens the browser to the authorize URL
// 3. Waits for the callback with the auth code
// 4. Exchanges the code for tokens
func Login(ctx context.Context, domain string, oauthCfg OAuthConfig, w io.Writer) (*OAuthToken, error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE challenge: %w", err)
	}

	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state parameter: %w", err)
	}

	// Start localhost server on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, callbackPath)

	// Channel to receive the auth code from the callback
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("login failed: state parameter mismatch (possible CSRF attack)")
			fmt.Fprint(w, htmlPage("Login failed", "Security check failed. You can close this tab and try again."))
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error_description")
			if errMsg == "" {
				errMsg = r.URL.Query().Get("error")
			}
			if errMsg == "" {
				errMsg = "no authorization code received"
			}
			errCh <- fmt.Errorf("login failed: %s", errMsg)
			fmt.Fprint(w, htmlPage("Login failed", "Something went wrong. You can close this tab and try again."))
			return
		}
		codeCh <- code
		fmt.Fprint(w, htmlPage("Login successful", "You can close this tab and return to the terminal."))
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()

	// Build authorize URL
	authorizeURL := fmt.Sprintf("https://api.%s/oauth/authorize?%s", domain, url.Values{
		"response_type":         {"code"},
		"client_id":             {oauthCfg.ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {oauthCfg.Scopes},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}.Encode())

	fmt.Fprintf(w, "Opening browser for login...\n")
	if err := openBrowser(authorizeURL); err != nil {
		fmt.Fprintf(w, "Could not open browser automatically.\nOpen this URL in your browser:\n\n  %s\n\n", authorizeURL)
	}
	fmt.Fprintf(w, "Waiting for login to complete...\n")

	// Wait for callback or timeout
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("login timed out after %s", loginTimeout)
	}

	// Exchange code for tokens
	return exchangeCode(ctx, domain, code, redirectURI, verifier, oauthCfg)
}

// Refresh exchanges a refresh token for a new access token.
func Refresh(ctx context.Context, domain, refreshToken string, oauthCfg OAuthConfig) (*OAuthToken, error) {
	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oauthCfg.ClientID},
		"refresh_token": {refreshToken},
	}

	return postTokenRequest(ctx, domain, data)
}

// Revoke revokes an access token.
func Revoke(ctx context.Context, domain, accessToken string, oauthCfg OAuthConfig) error {
	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()

	data := url.Values{
		"client_id": {oauthCfg.ClientID},
		"token":     {accessToken},
	}

	tokenURL := fmt.Sprintf("https://api.%s/oauth/revoke", domain)
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := getOAuthHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("revoke request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("revoke failed (HTTP %d)", resp.StatusCode)
	}
	return nil
}

func exchangeCode(ctx context.Context, domain, code, redirectURI, codeVerifier string, oauthCfg OAuthConfig) (*OAuthToken, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {oauthCfg.ClientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}

	return postTokenRequest(ctx, domain, data)
}

func postTokenRequest(ctx context.Context, domain string, data url.Values) (*OAuthToken, error) {
	tokenURL := fmt.Sprintf("https://api.%s/oauth/token", domain)
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := getOAuthHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Description != "" {
			return nil, fmt.Errorf("token exchange failed: %s", errResp.Description)
		}
		return nil, fmt.Errorf("token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		TokenType:    tokenResp.TokenType,
	}, nil
}

// generatePKCE creates a code verifier and its S256 challenge.
func generatePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}

// generateState creates a random state parameter for CSRF protection.
func generateState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

var openBrowser = func(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func htmlPage(title, message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><title>%s - Supermetrics CLI</title>
<style>body{font-family:sans-serif;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#f5f5f5}
.card{background:white;padding:2rem 3rem;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.1);text-align:center}
h1{color:#333;margin-bottom:0.5rem}p{color:#666}</style></head>
<body><div class="card"><h1>%s</h1><p>%s</p></div></body></html>`, title, title, message)
}
