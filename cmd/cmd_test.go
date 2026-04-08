package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/supermetrics-public/supermetrics-cli/internal/auth"
	"github.com/supermetrics-public/supermetrics-cli/internal/config"
	"github.com/supermetrics-public/supermetrics-cli/internal/exitcode"
	"github.com/supermetrics-public/supermetrics-cli/internal/httpclient"
)

func executeCommand(args ...string) (string, string, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()

	return stdout.String(), stderr.String(), err
}

func TestRootHelp(t *testing.T) {
	out, _, err := executeCommand("--help")
	require.NoError(t, err)

	expected := []string{
		"supermetrics",
		"login-links",
		"queries",
		"backfills",
		"accounts",
		"logins",
		"datasource",
		"version",
		"configure",
		"--api-key",
		"--output",
		"--verbose",
		"--fields",
		"--profile",
		"profile",
	}
	for _, s := range expected {
		assert.Contains(t, out, s, "help output should contain %q", s)
	}
}

func TestOutputFlagCompletion(t *testing.T) {
	fn, ok := rootCmd.GetFlagCompletionFunc("output")
	require.True(t, ok, "output flag should have a completion function registered")

	completions, directive := fn(rootCmd, nil, "")
	assert.Equal(t, []string{"json", "table", "csv"}, completions)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

func TestVersionCommand(t *testing.T) {
	// version command prints directly to os.Stdout via fmt.Printf,
	// so we just verify it runs without error
	_, _, err := executeCommand("version")
	require.NoError(t, err)
}

func TestVersionUpgradeHelp(t *testing.T) {
	out, _, err := executeCommand("version", "upgrade", "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "--check", "upgrade help should show --check flag")
	assert.Contains(t, out, "--force", "upgrade help should show --force flag")
}

func TestBackfillsHelp(t *testing.T) {
	out, _, err := executeCommand("backfills", "--help")
	require.NoError(t, err)

	expected := []string{"create", "get", "get-latest", "list-incomplete", "cancel"}
	for _, s := range expected {
		assert.Contains(t, out, s, "backfills help should contain subcommand %q", s)
	}
}

func TestQueriesExecuteHelp(t *testing.T) {
	out, _, err := executeCommand("queries", "execute", "--help")
	require.NoError(t, err)

	expected := []string{"--ds-id", "--fields", "--start-date", "--end-date", "--ds-accounts"}
	for _, s := range expected {
		assert.Contains(t, out, s, "queries execute help should contain flag %q", s)
	}
}

func TestLoginLinksHelp(t *testing.T) {
	out, _, err := executeCommand("login-links", "--help")
	require.NoError(t, err)

	expected := []string{"create", "get", "list", "close"}
	for _, s := range expected {
		assert.Contains(t, out, s, "login-links help should contain subcommand %q", s)
	}
}

func TestBackfillsCreateRequiresFlags(t *testing.T) {
	_, _, err := executeCommand("backfills", "create")
	require.Error(t, err)
}

func TestUnknownCommandError(t *testing.T) {
	_, _, err := executeCommand("nonexistent")
	require.Error(t, err)
}

func TestFormatDuration_OneMinute(t *testing.T) {
	got := formatDuration(time.Minute)
	assert.Equal(t, "1 minute", got)
}

func TestFormatDuration_MultipleMinutes(t *testing.T) {
	got := formatDuration(5 * time.Minute)
	assert.Equal(t, "5 minutes", got)
}

func TestFormatDuration_Hours(t *testing.T) {
	got := formatDuration(2 * time.Hour)
	assert.Equal(t, "120 minutes", got)
}

func TestMaskKey_Empty(t *testing.T) {
	got := maskKey("")
	assert.Equal(t, "not set", got)
}

func TestMaskKey_Short(t *testing.T) {
	got := maskKey("abcd")
	assert.Equal(t, "****", got)
}

func TestMaskKey_ExactlyEight(t *testing.T) {
	got := maskKey("12345678")
	assert.Equal(t, "****", got)
}

func TestMaskKey_Long(t *testing.T) {
	got := maskKey("sk_test_abc123xyz")
	assert.Equal(t, "sk_t****3xyz", got)
}

func TestReadLine_Normal(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("  hello world  \n"))
	got, err := readLine(reader)
	require.NoError(t, err)
	assert.Equal(t, "hello world", got)
}

func TestReadLine_EOF(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	got, err := readLine(reader)
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func withTempConfig(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	orig := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() { os.Setenv("XDG_CONFIG_HOME", orig) })
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
}

func TestPrintLoginStatus_NotLoggedIn(t *testing.T) {
	withTempConfig(t)

	var buf bytes.Buffer
	require.NoError(t, printLoginStatus(&buf))

	assert.Contains(t, buf.String(), "Not logged in")
}

func TestPrintLoginStatus_NotLoggedInWithAPIKey(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {APIKey: "sk_test_12345678"},
		},
	}))

	var buf bytes.Buffer
	require.NoError(t, printLoginStatus(&buf))

	out := buf.String()
	assert.Contains(t, out, "Not logged in")
	assert.Contains(t, out, "Using API key")
}

func TestPrintLoginStatus_ExpiredWithRefreshToken(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {
				AccessToken:  "expired-token",
				RefreshToken: "refresh-token",
				TokenExpiry:  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			},
		},
	}))

	var buf bytes.Buffer
	require.NoError(t, printLoginStatus(&buf))

	assert.Contains(t, buf.String(), "expired (will auto-refresh")
}

func TestPrintLoginStatus_ExpiredWithoutRefreshToken(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {
				AccessToken: "expired-token",
				TokenExpiry: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			},
		},
	}))

	var buf bytes.Buffer
	require.NoError(t, printLoginStatus(&buf))

	assert.Contains(t, buf.String(), "Run 'supermetrics login' to re-authenticate")
}

func TestPrintLoginStatus_ValidToken(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {
				AccessToken: "valid-token",
				TokenExpiry: time.Now().Add(30 * time.Minute).Format(time.RFC3339),
			},
		},
	}))

	var buf bytes.Buffer
	require.NoError(t, printLoginStatus(&buf))

	out := buf.String()
	assert.Contains(t, out, "Logged in via OAuth")
	assert.Contains(t, out, "minutes")
}

// --- Profile subcommand tests ---

func TestProfileList_Empty(t *testing.T) {
	withTempConfig(t)

	// "No profiles configured" is printed via infoWriter() which writes to os.Stdout,
	// not through cmd.OutOrStdout(). Just verify no error and no crash.
	_, _, err := executeCommand("profile", "list")
	require.NoError(t, err)
}

func TestProfileList_MultipleProfiles(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		ActiveProfile: "work",
		Profiles: map[string]*config.Profile{
			"default": {APIKey: "sk_test_12345678"},
			"work":    {AccessToken: "valid", TokenExpiry: "2099-01-01T00:00:00Z"},
			"staging": {},
		},
	}))

	out, _, err := executeCommand("profile", "list")
	require.NoError(t, err)

	// Active profile marked with *
	assert.Contains(t, out, "* work (OAuth)")
	// Others not marked
	assert.Contains(t, out, "  default (API key)")
	assert.Contains(t, out, "  staging (no credentials)")

	// Sorted alphabetically
	defaultIdx := strings.Index(out, "default")
	stagingIdx := strings.Index(out, "staging")
	workIdx := strings.Index(out, "work")
	assert.Less(t, defaultIdx, stagingIdx)
	assert.Less(t, stagingIdx, workIdx)
}

func TestProfileList_ExpiredOAuth(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {AccessToken: "expired", TokenExpiry: "2020-01-01T00:00:00Z"},
		},
	}))

	out, _, err := executeCommand("profile", "list")
	require.NoError(t, err)
	assert.Contains(t, out, "OAuth (expired)")
}

func TestProfileUse_Success(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {},
			"work":    {APIKey: "sk_test_workkey1"},
		},
	}))

	_, _, err := executeCommand("profile", "use", "work")
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "work", cfg.ActiveProfile)
}

func TestProfileUse_NotFound(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {},
		},
	}))

	_, _, err := executeCommand("profile", "use", "nonexistent")
	require.Error(t, err)
	var exitErr *exitcode.Error
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, exitcode.Usage, exitErr.Code)
}

func TestProfileDelete_Success(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {},
			"work":    {APIKey: "sk_test_workkey1"},
		},
	}))

	_, _, err := executeCommand("profile", "delete", "work")
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Nil(t, cfg.Profiles["work"])
	assert.NotNil(t, cfg.Profiles["default"])
}

func TestProfileDelete_ClearsActiveProfile(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		ActiveProfile: "work",
		Profiles: map[string]*config.Profile{
			"default": {},
			"work":    {APIKey: "sk_test_workkey1"},
		},
	}))

	_, _, err := executeCommand("profile", "delete", "work")
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.ActiveProfile)
}

func TestProfileDelete_NotFound(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {},
		},
	}))

	_, _, err := executeCommand("profile", "delete", "ghost")
	require.Error(t, err)
	var exitErr *exitcode.Error
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, exitcode.Usage, exitErr.Code)
}

func TestProfileShow_Default(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {APIKey: "sk_test_abc123xyz"},
		},
	}))

	out, _, err := executeCommand("profile", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "Profile: default")
	assert.Contains(t, out, "Status:  active")
	assert.Contains(t, out, "sk_t****3xyz") // masked
	assert.Contains(t, out, "OAuth:   not logged in")
}

func TestProfileShow_NamedProfile(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {},
			"work": {
				AccessToken: "valid-token",
				TokenExpiry: "2099-01-01T00:00:00Z",
			},
		},
	}))

	out, _, err := executeCommand("profile", "show", "work")
	require.NoError(t, err)
	assert.Contains(t, out, "Profile: work")
	assert.Contains(t, out, "OAuth:   authenticated")
}

func TestProfileShow_ExpiredOAuth(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {
				AccessToken: "expired",
				TokenExpiry: "2020-01-01T00:00:00Z",
			},
		},
	}))

	out, _, err := executeCommand("profile", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "OAuth:   expired")
}

func TestProfileShow_NotFound(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		Profiles: map[string]*config.Profile{
			"default": {},
		},
	}))

	_, _, err := executeCommand("profile", "show", "nope")
	require.Error(t, err)
	var exitErr *exitcode.Error
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, exitcode.Usage, exitErr.Code)
}

// --- GetProfile priority chain tests ---

func TestGetProfile_EnvOverridesConfig(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		ActiveProfile: "staging",
		Profiles: map[string]*config.Profile{
			"default": {},
			"staging": {},
			"work":    {},
		},
	}))

	// Reset flag so env takes effect
	flagProfile = ""
	t.Setenv("SUPERMETRICS_PROFILE", "work")

	assert.Equal(t, "work", GetProfile())
}

func TestGetProfile_ConfigActiveProfile(t *testing.T) {
	withTempConfig(t)
	require.NoError(t, config.Save(&config.Config{
		ActiveProfile: "staging",
		Profiles: map[string]*config.Profile{
			"default": {},
			"staging": {},
		},
	}))

	flagProfile = ""
	t.Setenv("SUPERMETRICS_PROFILE", "")

	// Empty env var should be ignored, falls through to config
	got := GetProfile()
	// With empty SUPERMETRICS_PROFILE set, LookupEnv returns ok=true but val=""
	// so it skips env, falls through to config active_profile
	assert.Equal(t, "staging", got)
}

func TestGetProfile_Default(t *testing.T) {
	withTempConfig(t)

	flagProfile = ""
	// Unset env entirely
	os.Unsetenv("SUPERMETRICS_PROFILE")

	assert.Equal(t, config.DefaultProfile, GetProfile())
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "generic error",
			err:  errors.New("something broke"),
			want: 1,
		},
		{
			name: "already wrapped with exit code",
			err:  exitcode.Wrap(errors.New("bad flag"), exitcode.Usage),
			want: exitcode.Usage,
		},
		{
			name: "API 401",
			err:  &httpclient.APIError{Message: "unauthorized", StatusCode: 401},
			want: exitcode.Auth,
		},
		{
			name: "API 403",
			err:  &httpclient.APIError{Message: "forbidden", StatusCode: 403},
			want: exitcode.Auth,
		},
		{
			name: "API 429",
			err:  &httpclient.APIError{Message: "rate limited", StatusCode: 429},
			want: exitcode.Unavailable,
		},
		{
			name: "API 500",
			err:  &httpclient.APIError{Message: "server error", StatusCode: 500},
			want: exitcode.Unavailable,
		},
		{
			name: "API 502",
			err:  &httpclient.APIError{Message: "bad gateway", StatusCode: 502},
			want: exitcode.Unavailable,
		},
		{
			name: "API 404 is generic",
			err:  &httpclient.APIError{Message: "not found", StatusCode: 404},
			want: 1,
		},
		{
			name: "no credentials sentinel",
			err:  auth.ErrNoCredentials,
			want: exitcode.Auth,
		},
		{
			name: "token expired sentinel",
			err:  auth.ErrTokenExpired,
			want: exitcode.Auth,
		},
		{
			name: "wrapped no credentials",
			err:  errors.Join(errors.New("resolving auth"), auth.ErrNoCredentials),
			want: exitcode.Auth,
		},
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: exitcode.Unavailable,
		},
		{
			name: "net.Error",
			err:  &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			want: exitcode.Unavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}
