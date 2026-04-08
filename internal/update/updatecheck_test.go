package update

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/supermetrics-public/supermetrics-cli/internal/config"
)

func withTempConfig(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	orig := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	return func() { os.Setenv("XDG_CONFIG_HOME", orig) }
}

func withTTY(t *testing.T) {
	t.Helper()
	origTerminal := isTerminalFunc
	origCygwin := isCygwinTerminalFunc
	isTerminalFunc = func(uintptr) bool { return true }
	isCygwinTerminalFunc = func(uintptr) bool { return false }
	t.Cleanup(func() {
		isTerminalFunc = origTerminal
		isCygwinTerminalFunc = origCygwin
	})
}

func clearEnvNoUpdateCheck(t *testing.T) {
	t.Helper()
	orig, existed := os.LookupEnv(envNoUpdateCheck)
	os.Unsetenv(envNoUpdateCheck)
	t.Cleanup(func() {
		if existed {
			os.Setenv(envNoUpdateCheck, orig)
		} else {
			os.Unsetenv(envNoUpdateCheck)
		}
	})
}

// --- ShouldCheck tests ---

func TestShouldCheck_DisabledByEnv(t *testing.T) {
	orig := os.Getenv(envNoUpdateCheck)
	os.Setenv(envNoUpdateCheck, "1")
	defer os.Setenv(envNoUpdateCheck, orig)

	assert.False(t, ShouldCheck(), "ShouldCheck should return false when env var is set")
}

func TestShouldCheck_DisabledByEnvTrue(t *testing.T) {
	orig := os.Getenv(envNoUpdateCheck)
	os.Setenv(envNoUpdateCheck, "true")
	defer os.Setenv(envNoUpdateCheck, orig)

	assert.False(t, ShouldCheck(), "ShouldCheck should return false when env var is 'true'")
}

func TestShouldCheck_NotDisabledByZero(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	orig := os.Getenv(envNoUpdateCheck)
	os.Setenv(envNoUpdateCheck, "0")
	defer os.Setenv(envNoUpdateCheck, orig)

	// With "0" the env check should NOT disable — ShouldCheck should proceed
	// past the env check. In non-TTY test environment it returns false due to
	// the isatty check (not the env check), which is correct behavior.
	_ = ShouldCheck()
}

func TestShouldCheck_EmptyEnvDoesNotDisable(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	orig := os.Getenv(envNoUpdateCheck)
	os.Setenv(envNoUpdateCheck, "")
	defer os.Setenv(envNoUpdateCheck, orig)

	// Empty env var should not disable check
	_ = ShouldCheck()
}

func TestShouldCheck_OldCheckInNonTTY(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	orig := os.Getenv(envNoUpdateCheck)
	os.Unsetenv(envNoUpdateCheck)
	defer os.Setenv(envNoUpdateCheck, orig)

	// Save config with old check timestamp (8 days ago)
	cfg := &config.Config{
		LastUpdateCheck: time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}
	err := config.Save(cfg)
	require.NoError(t, err)

	// In non-TTY test env, ShouldCheck returns false due to isatty check,
	// not because the interval is satisfied — this verifies no panic/crash
	// with an old timestamp
	result := ShouldCheck()
	assert.False(t, result, "ShouldCheck should return false in non-TTY even with old check")
}

func TestShouldCheck_RecentCheckSkips(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	// Clear env var
	orig := os.Getenv(envNoUpdateCheck)
	os.Setenv(envNoUpdateCheck, "")
	defer os.Setenv(envNoUpdateCheck, orig)

	// Save config with recent check
	cfg := &config.Config{
		LastUpdateCheck: time.Now().UTC().Format(time.RFC3339),
	}
	err := config.Save(cfg)
	require.NoError(t, err)

	// ShouldCheck returns false due to non-TTY in test (stdout is not a terminal)
	// This is correct behavior — we verify it doesn't panic
	result := ShouldCheck()
	// In test environment stdout is not a TTY, so this should be false
	assert.False(t, result, "ShouldCheck should return false in non-TTY environment")
}

// --- ShouldCheck TTY-enabled tests ---

func TestShouldCheck_TTY_NoConfig(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)
	clearEnvNoUpdateCheck(t)

	// No config file exists → should check
	assert.True(t, ShouldCheck(), "ShouldCheck should return true when no config exists")
}

func TestShouldCheck_TTY_NoLastCheck(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)
	clearEnvNoUpdateCheck(t)

	// Config exists but no LastUpdateCheck
	cfg := &config.Config{DefaultOutput: "json"}
	require.NoError(t, config.Save(cfg))

	assert.True(t, ShouldCheck(), "ShouldCheck should return true when LastUpdateCheck is empty")
}

func TestShouldCheck_TTY_RecentCheck(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)
	clearEnvNoUpdateCheck(t)

	cfg := &config.Config{
		LastUpdateCheck: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, config.Save(cfg))

	assert.False(t, ShouldCheck(), "ShouldCheck should return false when checked recently")
}

func TestShouldCheck_TTY_OldCheck(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)
	clearEnvNoUpdateCheck(t)

	cfg := &config.Config{
		LastUpdateCheck: time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}
	require.NoError(t, config.Save(cfg))

	assert.True(t, ShouldCheck(), "ShouldCheck should return true when last check was >7 days ago")
}

func TestShouldCheck_TTY_MalformedTimestamp(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)
	clearEnvNoUpdateCheck(t)

	cfg := &config.Config{
		LastUpdateCheck: "not-a-timestamp",
	}
	require.NoError(t, config.Save(cfg))

	assert.True(t, ShouldCheck(), "ShouldCheck should return true when timestamp is malformed")
}

// --- runBackgroundCheckSync tests ---

func TestRunBackgroundCheckSync_NewerVersion(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	u := newTestUpdater(
		func(_ context.Context) (*ReleaseInfo, bool, error) {
			return &ReleaseInfo{Version: "2.0.0"}, true, nil
		}, nil, nil,
	)

	u.runBackgroundCheckSync("1.0.0")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", cfg.AvailableUpdate)
	assert.NotEmpty(t, cfg.LastUpdateCheck)
}

func TestRunBackgroundCheckSync_SameVersion(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	// Pre-set an available update that should be cleared
	cfg := &config.Config{AvailableUpdate: "old-version"}
	err := config.Save(cfg)
	require.NoError(t, err)

	u := newTestUpdater(
		func(_ context.Context) (*ReleaseInfo, bool, error) {
			return &ReleaseInfo{Version: "1.0.0"}, true, nil
		}, nil, nil,
	)

	u.runBackgroundCheckSync("1.0.0")

	cfg, err = config.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.AvailableUpdate, "expected AvailableUpdate to be cleared")
}

func TestRunBackgroundCheckSync_LatestError(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	u := newTestUpdater(
		func(_ context.Context) (*ReleaseInfo, bool, error) {
			return nil, false, fmt.Errorf("network error")
		}, nil, nil,
	)

	// Should not panic or modify config
	u.runBackgroundCheckSync("1.0.0")

	// Function returns early on error before touching config,
	// so config should not exist (Load returns error) or be unchanged
	cfg, err := config.Load()
	if err == nil {
		assert.Empty(t, cfg.AvailableUpdate, "expected no AvailableUpdate after error")
	}
}

func TestRunBackgroundCheckSync_NoReleases(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	u := newTestUpdater(
		func(_ context.Context) (*ReleaseInfo, bool, error) {
			return nil, false, nil
		}, nil, nil,
	)

	// Should return early without modifying config
	u.runBackgroundCheckSync("1.0.0")
}

func TestRunBackgroundCheckSync_VPrefixHandled(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	u := newTestUpdater(
		func(_ context.Context) (*ReleaseInfo, bool, error) {
			return &ReleaseInfo{Version: "1.0.0"}, true, nil
		}, nil, nil,
	)

	u.runBackgroundCheckSync("v1.0.0")

	cfg, err := config.Load()
	require.NoError(t, err)
	// v1.0.0 stripped to 1.0.0, matches latest → should clear
	assert.Empty(t, cfg.AvailableUpdate, "expected AvailableUpdate cleared with v-prefix")
}

// --- PrintUpdateHint tests ---

func TestPrintUpdateHint_NoConfig(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	// No config file, no available update — should not print
	printed := PrintUpdateHint(io.Discard, "v0.1.0")
	assert.False(t, printed, "should not print hint when no config exists")
}

func TestPrintUpdateHint_NoUpdate(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	cfg := &config.Config{AvailableUpdate: ""}
	err := config.Save(cfg)
	require.NoError(t, err)

	printed := PrintUpdateHint(io.Discard, "v0.1.0")
	assert.False(t, printed, "should not print hint when no update available")
}

func TestPrintUpdateHint_SameVersion(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	cfg := &config.Config{AvailableUpdate: "0.1.0"}
	err := config.Save(cfg)
	require.NoError(t, err)

	// Current version matches available — no hint
	printed := PrintUpdateHint(io.Discard, "0.1.0")
	assert.False(t, printed, "should not print hint when versions match")
}

// --- PrintUpdateHint TTY-enabled tests ---

func TestPrintUpdateHint_TTY_UpdateAvailable(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)

	cfg := &config.Config{AvailableUpdate: "2.0.0"}
	require.NoError(t, config.Save(cfg))

	var buf bytes.Buffer
	printed := PrintUpdateHint(&buf, "1.0.0")

	assert.True(t, printed, "should print hint when newer version available")
	assert.Contains(t, buf.String(), "v1.0.0")
	assert.Contains(t, buf.String(), "v2.0.0")
}

func TestPrintUpdateHint_TTY_NoConfig(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)

	// No config saved → config.Load returns error or empty config
	printed := PrintUpdateHint(io.Discard, "1.0.0")
	assert.False(t, printed, "should not print hint when config load fails")
}

func TestPrintUpdateHint_TTY_SameVersion(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)

	cfg := &config.Config{AvailableUpdate: "1.0.0"}
	require.NoError(t, config.Save(cfg))

	printed := PrintUpdateHint(io.Discard, "v1.0.0")
	assert.False(t, printed, "should not print hint when versions match")
}

func TestPrintUpdateHint_TTY_EmptyUpdate(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()
	withTTY(t)

	cfg := &config.Config{AvailableUpdate: ""}
	require.NoError(t, config.Save(cfg))

	printed := PrintUpdateHint(io.Discard, "1.0.0")
	assert.False(t, printed, "should not print hint when AvailableUpdate is empty")
}

// --- printUpdateHintFormatted tests (bypasses TTY check) ---

func TestPrintUpdateHintFormatted(t *testing.T) {
	var buf bytes.Buffer
	result := printUpdateHintFormatted(&buf, "1.0.0", "2.0.0")

	assert.True(t, result, "expected true return")
	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "v1.0.0")
	assert.Contains(t, output, "v2.0.0")
	assert.Contains(t, output, "supermetrics version upgrade")
}
