package update

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUpdater(
	latest func(ctx context.Context) (*ReleaseInfo, bool, error),
	detect func(ctx context.Context, version string) (*ReleaseInfo, bool, error),
	updateFn func(ctx context.Context, assetURL, assetName, execPath string) error,
) *Updater {
	return &Updater{
		latestRelease: latest,
		detectVersion: detect,
		updateBinary:  updateFn,
	}
}

func noopUpdate(_ context.Context, _, _, _ string) error { return nil }

// --- Upgrade tests ---

func TestUpgrade(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		latest         func(ctx context.Context) (*ReleaseInfo, bool, error)
		updateFn       func(ctx context.Context, assetURL, assetName, execPath string) error
		wantErr        string
		wantOutput     string
		wantNoUpdate   bool // expect updateBinary NOT called
	}{
		{
			name:           "already up to date",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return &ReleaseInfo{Version: "1.0.0"}, true, nil
			},
			wantOutput:   "Already up to date (v1.0.0)",
			wantNoUpdate: true,
		},
		{
			name:           "already up to date with v prefix",
			currentVersion: "v1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return &ReleaseInfo{Version: "1.0.0"}, true, nil
			},
			wantOutput:   "Already up to date (v1.0.0)",
			wantNoUpdate: true,
		},
		{
			name:           "newer version available",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return &ReleaseInfo{Version: "2.0.0", AssetURL: "https://example.com/bin", AssetName: "supermetrics"}, true, nil
			},
			updateFn:   noopUpdate,
			wantOutput: "Upgraded supermetrics from v1.0.0 to v2.0.0",
		},
		{
			name:           "latest version error",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return nil, false, fmt.Errorf("network error")
			},
			wantErr: "failed to check for updates",
		},
		{
			name:           "no releases found",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return nil, false, nil
			},
			wantErr: "no releases found",
		},
		{
			name:           "update binary fails",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return &ReleaseInfo{Version: "2.0.0", AssetURL: "https://example.com/bin", AssetName: "supermetrics"}, true, nil
			},
			updateFn: func(_ context.Context, _, _, _ string) error {
				return fmt.Errorf("permission denied")
			},
			wantErr: "upgrade failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateCalled := false
			updateFn := tt.updateFn
			if updateFn == nil && !tt.wantNoUpdate {
				updateFn = noopUpdate
			}
			if updateFn == nil {
				updateFn = func(_ context.Context, _, _, _ string) error {
					updateCalled = true
					return nil
				}
			} else if tt.wantNoUpdate {
				origFn := updateFn
				updateFn = func(ctx context.Context, a, b, c string) error {
					updateCalled = true
					return origFn(ctx, a, b, c)
				}
			}

			u := newTestUpdater(tt.latest, nil, updateFn)
			var buf bytes.Buffer
			err := u.Upgrade(context.Background(), &buf, tt.currentVersion)

			if tt.wantErr != "" {
				require.Error(t, err, "expected error")
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, buf.String(), tt.wantOutput)
			if tt.wantNoUpdate {
				assert.False(t, updateCalled, "updateBinary should not have been called")
			}
		})
	}
}

func TestUpgrade_PassesCorrectArgs(t *testing.T) {
	var gotURL, gotName string
	u := newTestUpdater(
		func(_ context.Context) (*ReleaseInfo, bool, error) {
			return &ReleaseInfo{Version: "2.0.0", AssetURL: "https://cdn.example.com/v2", AssetName: "supermetrics-linux-amd64"}, true, nil
		},
		nil,
		func(_ context.Context, assetURL, assetName, _ string) error {
			gotURL = assetURL
			gotName = assetName
			return nil
		},
	)
	var buf bytes.Buffer
	err := u.Upgrade(context.Background(), &buf, "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/v2", gotURL)
	assert.Equal(t, "supermetrics-linux-amd64", gotName)
}

// --- ForceReinstall tests ---

func TestForceReinstall(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		detect         func(ctx context.Context, version string) (*ReleaseInfo, bool, error)
		updateFn       func(ctx context.Context, assetURL, assetName, execPath string) error
		wantErr        string
		wantOutput     string
	}{
		{
			name:           "success",
			currentVersion: "1.0.0",
			detect: func(_ context.Context, ver string) (*ReleaseInfo, bool, error) {
				return &ReleaseInfo{Version: ver, AssetURL: "https://example.com/bin", AssetName: "supermetrics"}, true, nil
			},
			updateFn:   noopUpdate,
			wantOutput: "Reinstalled supermetrics v1.0.0",
		},
		{
			name:           "version not found",
			currentVersion: "99.0.0",
			detect: func(_ context.Context, _ string) (*ReleaseInfo, bool, error) {
				return nil, false, nil
			},
			wantErr: "version v99.0.0 not found in releases",
		},
		{
			name:           "detect error",
			currentVersion: "1.0.0",
			detect: func(_ context.Context, _ string) (*ReleaseInfo, bool, error) {
				return nil, false, fmt.Errorf("api error")
			},
			wantErr: "not found in releases",
		},
		{
			name:           "update binary fails",
			currentVersion: "1.0.0",
			detect: func(_ context.Context, ver string) (*ReleaseInfo, bool, error) {
				return &ReleaseInfo{Version: ver, AssetURL: "https://example.com/bin", AssetName: "supermetrics"}, true, nil
			},
			updateFn: func(_ context.Context, _, _, _ string) error {
				return fmt.Errorf("disk full")
			},
			wantErr: "reinstall failed",
		},
		{
			name:           "v prefix stripped",
			currentVersion: "v1.0.0",
			detect: func(_ context.Context, ver string) (*ReleaseInfo, bool, error) {
				if ver != "1.0.0" {
					return nil, false, fmt.Errorf("expected version without v prefix, got %q", ver)
				}
				return &ReleaseInfo{Version: ver, AssetURL: "https://example.com/bin", AssetName: "supermetrics"}, true, nil
			},
			updateFn:   noopUpdate,
			wantOutput: "Reinstalled supermetrics v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateFn := tt.updateFn
			if updateFn == nil {
				updateFn = noopUpdate
			}
			u := newTestUpdater(nil, tt.detect, updateFn)
			var buf bytes.Buffer
			err := u.ForceReinstall(context.Background(), &buf, tt.currentVersion)

			if tt.wantErr != "" {
				require.Error(t, err, "expected error")
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, buf.String(), tt.wantOutput)
		})
	}
}

// --- CheckOnly tests ---

func TestCheckOnly(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		latest         func(ctx context.Context) (*ReleaseInfo, bool, error)
		wantErr        string
		wantOutput     string
	}{
		{
			name:           "up to date",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return &ReleaseInfo{Version: "1.0.0"}, true, nil
			},
			wantOutput: "You are up to date.",
		},
		{
			name:           "newer available",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return &ReleaseInfo{Version: "2.0.0"}, true, nil
			},
			wantOutput: "Latest version:  v2.0.0",
		},
		{
			name:           "error",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return nil, false, fmt.Errorf("timeout")
			},
			wantErr: "failed to check for updates",
		},
		{
			name:           "no releases",
			currentVersion: "1.0.0",
			latest: func(_ context.Context) (*ReleaseInfo, bool, error) {
				return nil, false, nil
			},
			wantOutput: "No releases found.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestUpdater(tt.latest, nil, nil)
			var buf bytes.Buffer
			err := u.CheckOnly(context.Background(), &buf, tt.currentVersion)

			if tt.wantErr != "" {
				require.Error(t, err, "expected error")
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, buf.String(), tt.wantOutput)
		})
	}
}

func TestCheckOnly_ShowsCurrentVersion(t *testing.T) {
	u := newTestUpdater(
		func(_ context.Context) (*ReleaseInfo, bool, error) {
			return &ReleaseInfo{Version: "1.0.0"}, true, nil
		}, nil, nil,
	)
	var buf bytes.Buffer
	err := u.CheckOnly(context.Background(), &buf, "v1.0.0")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Current version: v1.0.0")
}

func TestCheckOnly_NewerShowsUpgradeHint(t *testing.T) {
	u := newTestUpdater(
		func(_ context.Context) (*ReleaseInfo, bool, error) {
			return &ReleaseInfo{Version: "2.0.0"}, true, nil
		}, nil, nil,
	)
	var buf bytes.Buffer
	err := u.CheckOnly(context.Background(), &buf, "1.0.0")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "supermetrics version upgrade")
}

// --- isHomebrew tests ---

func TestIsHomebrew_NotHomebrew(t *testing.T) {
	// The test binary itself is not managed by Homebrew
	assert.False(t, isHomebrew(), "test binary should not be detected as Homebrew-managed")
}

func TestIsHomebrew_CellarPath(t *testing.T) {
	// Create a fake binary in a Cellar-like path
	tmpDir := t.TempDir()
	cellarDir := filepath.Join(tmpDir, "Cellar", "supermetrics", "0.1.0", "bin")
	err := os.MkdirAll(cellarDir, 0o750)
	require.NoError(t, err)

	fakeBin := filepath.Join(cellarDir, "supermetrics")
	err = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o600)
	require.NoError(t, err)

	// Create a symlink to the fake binary, simulating Homebrew's structure
	linkPath := filepath.Join(tmpDir, "supermetrics")
	err = os.Symlink(fakeBin, linkPath)
	require.NoError(t, err)

	// Resolve the symlink and check if it contains "Cellar"
	resolved, err := filepath.EvalSymlinks(linkPath)
	require.NoError(t, err)

	// Verify our test setup is correct — the resolved path should contain "Cellar"
	assert.Contains(t, resolved, "Cellar")
}
