package update

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

const (
	// GitHubOwner is the GitHub org/user that owns the CLI repo.
	GitHubOwner = "supermetrics-public"
	// GitHubRepo is the GitHub repository name.
	GitHubRepo = "supermetrics-cli"
)

var repo = selfupdate.NewRepositorySlug(GitHubOwner, GitHubRepo)

// ReleaseInfo holds the fields we need from a release.
// Exists because go-selfupdate's Release has an unexported version field,
// making it impossible to construct in tests.
type ReleaseInfo struct {
	Version   string // e.g. "1.2.3" (no "v" prefix)
	AssetURL  string
	AssetName string
}

// Updater performs version checks and self-updates. The function fields
// are injectable for testing; NewUpdater wires the real go-selfupdate calls.
type Updater struct {
	latestRelease func(ctx context.Context) (*ReleaseInfo, bool, error)
	detectVersion func(ctx context.Context, version string) (*ReleaseInfo, bool, error)
	updateBinary  func(ctx context.Context, assetURL, assetName, execPath string) error
}

// NewUpdater creates an Updater wired to real GitHub Releases via go-selfupdate.
func NewUpdater() *Updater {
	return &Updater{
		latestRelease: func(ctx context.Context) (*ReleaseInfo, bool, error) {
			rel, found, err := selfupdate.DetectLatest(ctx, repo)
			if err != nil || !found {
				return nil, found, err
			}
			return &ReleaseInfo{
				Version:   rel.Version(),
				AssetURL:  rel.AssetURL,
				AssetName: rel.AssetName,
			}, true, nil
		},
		detectVersion: func(ctx context.Context, version string) (*ReleaseInfo, bool, error) {
			rel, found, err := selfupdate.DetectVersion(ctx, repo, version)
			if err != nil || !found {
				return nil, found, err
			}
			return &ReleaseInfo{
				Version:   rel.Version(),
				AssetURL:  rel.AssetURL,
				AssetName: rel.AssetName,
			}, true, nil
		},
		updateBinary: selfupdate.UpdateTo,
	}
}

// Upgrade downloads and installs the latest version, replacing the current binary.
// currentVersion should be the version without "v" prefix (e.g. "0.2.0").
func (u *Updater) Upgrade(ctx context.Context, w io.Writer, currentVersion string) error {
	if isHomebrew() {
		return fmt.Errorf("this installation is managed by Homebrew. Run 'brew upgrade supermetrics' instead")
	}

	latest, found, err := u.latestRelease(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	if !found {
		return fmt.Errorf("no releases found")
	}

	currentClean := strings.TrimPrefix(currentVersion, "v")
	if latest.Version == currentClean {
		fmt.Fprintf(w, "Already up to date (v%s)\n", currentClean)
		return nil
	}

	fmt.Fprintf(w, "Upgrading supermetrics from v%s to v%s...\n", currentClean, latest.Version)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find current executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	if err := u.updateBinary(ctx, latest.AssetURL, latest.AssetName, exe); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	fmt.Fprintf(w, "Upgraded supermetrics from v%s to v%s\n", currentClean, latest.Version)
	return nil
}

// ForceReinstall downloads and reinstalls the current version.
func (u *Updater) ForceReinstall(ctx context.Context, w io.Writer, currentVersion string) error {
	if isHomebrew() {
		return fmt.Errorf("this installation is managed by Homebrew. Run 'brew reinstall supermetrics' instead")
	}

	currentClean := strings.TrimPrefix(currentVersion, "v")

	release, found, err := u.detectVersion(ctx, currentClean)
	if err != nil || !found {
		return fmt.Errorf("version v%s not found in releases", currentClean)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find current executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	fmt.Fprintf(w, "Reinstalling supermetrics v%s...\n", currentClean)
	if err := u.updateBinary(ctx, release.AssetURL, release.AssetName, exe); err != nil {
		return fmt.Errorf("reinstall failed: %w", err)
	}

	fmt.Fprintf(w, "Reinstalled supermetrics v%s\n", currentClean)
	return nil
}

// CheckOnly prints version comparison without installing.
func (u *Updater) CheckOnly(ctx context.Context, w io.Writer, currentVersion string) error {
	currentClean := strings.TrimPrefix(currentVersion, "v")
	fmt.Fprintf(w, "Current version: v%s\n", currentClean)

	latest, found, err := u.latestRelease(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	if !found {
		fmt.Fprintln(w, "No releases found.")
		return nil
	}

	if latest.Version == currentClean {
		fmt.Fprintln(w, "You are up to date.")
	} else {
		fmt.Fprintf(w, "Latest version:  v%s\n", latest.Version)
		fmt.Fprintln(w, "Run 'supermetrics version upgrade' to install.")
	}
	return nil
}

// isHomebrew returns true if the binary appears to be managed by Homebrew.
func isHomebrew() bool {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return false
	}
	return strings.Contains(resolved, "Cellar") || strings.Contains(resolved, "homebrew")
}
