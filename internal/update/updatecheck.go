package update

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/supermetrics-public/supermetrics-cli/internal/config"
)

const envNoUpdateCheck = "SUPERMETRICS_NO_UPDATE_CHECK"

// Package-level function variables for TTY detection, replaceable in tests.
var (
	isTerminalFunc       = isatty.IsTerminal
	isCygwinTerminalFunc = isatty.IsCygwinTerminal
)

// ShouldCheck returns true if a background update check should run.
// Returns false if: non-interactive, disabled via env, or checked recently.
func ShouldCheck() bool {
	// Disabled via env var
	if val, ok := os.LookupEnv(envNoUpdateCheck); ok && val != "" && val != "0" {
		return false
	}

	// Non-interactive (piped/redirected stdout)
	if !isTerminalFunc(os.Stdout.Fd()) && !isCygwinTerminalFunc(os.Stdout.Fd()) {
		return false
	}

	// Check interval
	cfg, err := config.Load()
	if err != nil {
		return true // If we can't load config, check anyway
	}

	if cfg.LastUpdateCheck == "" {
		return true
	}

	lastCheck, err := time.Parse(time.RFC3339, cfg.LastUpdateCheck)
	if err != nil {
		return true
	}

	return time.Since(lastCheck) >= cfg.UpdateCheckInterval()
}

// RunBackgroundCheck spawns a non-blocking goroutine that checks for updates.
// If a newer version is found, it writes the version to the config file
// for display on the next invocation.
func (u *Updater) RunBackgroundCheck(currentVersion string) {
	go u.runBackgroundCheckSync(currentVersion)
}

// runBackgroundCheckSync performs the update check synchronously.
// Extracted from RunBackgroundCheck for testability.
func (u *Updater) runBackgroundCheckSync(currentVersion string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	latest, found, err := u.latestRelease(ctx)
	if err != nil || !found {
		return // Silently ignore failures
	}

	// Update last check timestamp
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}
	cfg.LastUpdateCheck = time.Now().UTC().Format(time.RFC3339)

	currentClean := strings.TrimPrefix(currentVersion, "v")

	if latest.Version != currentClean {
		cfg.AvailableUpdate = latest.Version
	} else {
		cfg.AvailableUpdate = ""
	}

	_ = config.Save(cfg) // Best effort
}

// PrintUpdateHint prints a one-line update notice if a newer version is
// available (from a previous background check). Returns true if a hint was printed.
func PrintUpdateHint(w io.Writer, currentVersion string) bool {
	// Don't print in non-interactive mode
	if !isTerminalFunc(os.Stderr.Fd()) && !isCygwinTerminalFunc(os.Stderr.Fd()) {
		return false
	}

	cfg, err := config.Load()
	if err != nil || cfg.AvailableUpdate == "" {
		return false
	}

	currentClean := strings.TrimPrefix(currentVersion, "v")
	if cfg.AvailableUpdate == currentClean {
		return false
	}

	return printUpdateHintFormatted(w, currentClean, cfg.AvailableUpdate)
}

// printUpdateHintFormatted writes the update hint message.
// Extracted from PrintUpdateHint so it can be tested without a TTY.
func printUpdateHintFormatted(w io.Writer, currentVersion, availableVersion string) bool {
	fmt.Fprintf(w, "A new version of supermetrics is available (v%s → v%s). Run \"supermetrics version upgrade\" to upgrade.\n",
		currentVersion, availableVersion)
	return true
}
