package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// currentAssetName returns the release archive name for the running platform,
// matching the goreleaser naming scheme.
func currentAssetName(version string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("supermetrics-cli_%s_%s_%s%s", version, runtime.GOOS, runtime.GOARCH, ext)
}

// serveGitHub points githubAPIBase at a test server for the duration of the test.
func serveGitHub(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	orig := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() {
		githubAPIBase = orig
		srv.Close()
	})
	return srv
}

func releaseJSON(version string) string {
	return fmt.Sprintf(`{
		"tag_name": "v%[1]s",
		"assets": [
			{"name": %[2]q, "browser_download_url": "https://cdn.example.com/%[2]s"},
			{"name": "checksums.txt", "browser_download_url": "https://cdn.example.com/checksums.txt"},
			{"name": "supermetrics_%[1]s_linux_amd64.deb", "browser_download_url": "https://cdn.example.com/deb"}
		]
	}`, version, currentAssetName(version))
}

func TestFetchRelease_Latest(t *testing.T) {
	var gotPath, gotAccept, gotUA string
	serveGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		fmt.Fprint(w, releaseJSON("1.2.3"))
	})

	rel, found, err := fetchRelease(context.Background(), newHTTPClient(), "/releases/latest")
	require.NoError(t, err)
	require.True(t, found)

	assert.Equal(t, "/repos/supermetrics-public/supermetrics-cli/releases/latest", gotPath)
	assert.Equal(t, "application/vnd.github+json", gotAccept)
	assert.Contains(t, gotUA, "supermetrics-cli/")
	assert.Equal(t, "v1.2.3", rel.TagName)
	assert.Len(t, rel.Assets, 3)
}

func TestFetchRelease_NotFound(t *testing.T) {
	serveGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	rel, found, err := fetchRelease(context.Background(), newHTTPClient(), "/releases/tags/v9.9.9")
	require.NoError(t, err, "404 means no such release, not a failure")
	assert.False(t, found)
	assert.Nil(t, rel)
}

func TestFetchRelease_RateLimited(t *testing.T) {
	serveGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1767225600")
		w.WriteHeader(http.StatusForbidden)
	})

	_, found, err := fetchRelease(context.Background(), newHTTPClient(), "/releases/latest")
	require.Error(t, err)
	assert.False(t, found)
	assert.ErrorContains(t, err, "rate limit")
	assert.ErrorContains(t, err, "resets at")
}

func TestFetchRelease_ServerError(t *testing.T) {
	serveGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, found, err := fetchRelease(context.Background(), newHTTPClient(), "/releases/latest")
	require.Error(t, err)
	assert.False(t, found)
	assert.ErrorContains(t, err, "401")
}

func TestFetchRelease_MalformedJSON(t *testing.T) {
	serveGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not json")
	})

	_, _, err := fetchRelease(context.Background(), newHTTPClient(), "/releases/latest")
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse")
}

func TestReleaseInfo_SelectsPlatformAsset(t *testing.T) {
	asset := currentAssetName("1.2.3")
	rel := &ghRelease{
		TagName: "v1.2.3",
		Assets: []ghAsset{
			{Name: "supermetrics_1.2.3_linux_amd64.deb", URL: "https://cdn.example.com/deb"},
			{Name: "supermetrics_1.2.3_linux_arm64.rpm", URL: "https://cdn.example.com/rpm"},
			{Name: asset, URL: "https://cdn.example.com/archive"},
			{Name: "checksums.txt", URL: "https://cdn.example.com/checksums.txt"},
		},
	}

	info, err := releaseInfo(rel)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", info.Version, "v prefix stripped")
	assert.Equal(t, asset, info.AssetName)
	assert.Equal(t, "https://cdn.example.com/archive", info.AssetURL)
	assert.Equal(t, "https://cdn.example.com/checksums.txt", info.ChecksumURL)
}

func TestReleaseInfo_NoAssetForPlatform(t *testing.T) {
	rel := &ghRelease{
		TagName: "v1.2.3",
		Assets: []ghAsset{
			{Name: "supermetrics-cli_1.2.3_plan9_mips.tar.gz", URL: "https://cdn.example.com/other"},
			{Name: "checksums.txt", URL: "https://cdn.example.com/checksums.txt"},
		},
	}

	_, err := releaseInfo(rel)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no asset for "+runtime.GOOS)
}

func TestReleaseInfo_NoChecksums(t *testing.T) {
	rel := &ghRelease{
		TagName: "v1.2.3",
		Assets:  []ghAsset{{Name: currentAssetName("1.2.3"), URL: "https://cdn.example.com/archive"}},
	}

	_, err := releaseInfo(rel)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no checksums.txt")
}

func TestNewUpdater_LatestAndVersionPaths(t *testing.T) {
	var paths []string
	serveGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		fmt.Fprint(w, releaseJSON("1.2.3"))
	})

	u := NewUpdater()

	latest, found, err := u.latestRelease(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "1.2.3", latest.Version)

	_, found, err = u.detectVersion(context.Background(), "v1.2.3")
	require.NoError(t, err)
	assert.True(t, found)

	assert.Equal(t, []string{
		"/repos/supermetrics-public/supermetrics-cli/releases/latest",
		"/repos/supermetrics-public/supermetrics-cli/releases/tags/v1.2.3",
	}, paths, "version lookup must not double the v prefix")
}
