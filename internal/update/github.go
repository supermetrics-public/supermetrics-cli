package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/supermetrics-public/supermetrics-cli/internal/buildcfg"
	"github.com/supermetrics-public/supermetrics-cli/internal/httpclient"
)

const (
	githubAPIVersion  = "2022-11-28"
	checksumAssetName = "checksums.txt"
	releaseTimeout    = 30 * time.Second
	maxResponseBytes  = 64 << 20 // 64 MiB, guards against oversized release assets
)

// githubAPIBase is a variable so tests can point it at a local server.
var githubAPIBase = "https://api.github.com"

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

func userAgent() string {
	return "supermetrics-cli/" + buildcfg.Version
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   releaseTimeout,
		Transport: httpclient.Retry(httpclient.RetryConfig{})(http.DefaultTransport),
	}
}

// fetchRelease requests a release endpoint (e.g. "/releases/latest") and reports
// whether it exists. A 404 means no such release, not an error.
func fetchRelease(ctx context.Context, client *http.Client, path string) (*ghRelease, bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s%s", githubAPIBase, GitHubOwner, GitHubRepo, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to build release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	req.Header.Set("User-Agent", userAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to reach GitHub: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, false, nil
	case isRateLimited(resp):
		return nil, false, fmt.Errorf("GitHub API rate limit exceeded%s", rateLimitResetHint(resp))
	case resp.StatusCode != http.StatusOK:
		return nil, false, fmt.Errorf("GitHub returned %s", resp.Status)
	}

	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&rel); err != nil {
		return nil, false, fmt.Errorf("failed to parse GitHub release: %w", err)
	}
	return &rel, true, nil
}

func isRateLimited(resp *http.Response) bool {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return false
	}
	return resp.Header.Get("X-RateLimit-Remaining") == "0"
}

func rateLimitResetHint(resp *http.Response) string {
	epoch, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(", resets at %s", time.Unix(epoch, 0).Local().Format(time.Kitchen))
}

// releaseInfo maps a GitHub release to the asset for the running platform.
func releaseInfo(rel *ghRelease) (*ReleaseInfo, error) {
	suffix := assetSuffix()

	info := &ReleaseInfo{Version: strings.TrimPrefix(rel.TagName, "v")}
	for _, asset := range rel.Assets {
		switch {
		case strings.HasSuffix(asset.Name, suffix):
			info.AssetURL = asset.URL
			info.AssetName = asset.Name
		case asset.Name == checksumAssetName:
			info.ChecksumURL = asset.URL
		}
	}

	if info.AssetURL == "" {
		return nil, fmt.Errorf("release %s has no asset for %s/%s", rel.TagName, runtime.GOOS, runtime.GOARCH)
	}
	if info.ChecksumURL == "" {
		return nil, fmt.Errorf("release %s has no %s", rel.TagName, checksumAssetName)
	}
	return info, nil
}

func assetSuffix() string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
}
