package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeBinary = "#!/bin/sh\necho new version\n"

func platformBinaryName() string {
	if runtime.GOOS == "windows" {
		return binaryName + ".exe"
	}
	return binaryName
}

func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o755,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// serveRelease serves an archive and a goreleaser-style checksums file.
func serveRelease(t *testing.T, assetName string, archive []byte, checksums string) *ReleaseInfo {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/"+assetName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, checksums)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &ReleaseInfo{
		Version:     "2.0.0",
		AssetName:   assetName,
		AssetURL:    srv.URL + "/" + assetName,
		ChecksumURL: srv.URL + "/checksums.txt",
	}
}

func targetFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), platformBinaryName())
	require.NoError(t, os.WriteFile(path, []byte("old version"), 0o600))
	return path
}

func TestApplyUpdate_TarGz(t *testing.T) {
	assetName := currentAssetName("2.0.0")
	archive := makeTarGz(t, map[string]string{
		"README.md":          "# docs",
		platformBinaryName(): fakeBinary,
	})
	checksums := fmt.Sprintf("%s  %s\n%s  other.tar.gz\n", sha256Hex(archive), assetName, sha256Hex([]byte("x")))
	rel := serveRelease(t, assetName, archive, checksums)
	target := targetFile(t)

	require.NoError(t, applyUpdate(context.Background(), newHTTPClient(), rel, target))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, fakeBinary, string(got), "binary should be replaced with the archive contents")
}

func TestApplyUpdate_Zip(t *testing.T) {
	assetName := "supermetrics-cli_2.0.0_windows_amd64.zip"
	archive := makeZip(t, map[string]string{
		"README.md":          "# docs",
		platformBinaryName(): fakeBinary,
	})
	checksums := fmt.Sprintf("%s  %s\n", sha256Hex(archive), assetName)
	rel := serveRelease(t, assetName, archive, checksums)
	target := targetFile(t)

	require.NoError(t, applyUpdate(context.Background(), newHTTPClient(), rel, target))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, fakeBinary, string(got))
}

func TestApplyUpdate_ChecksumMismatch(t *testing.T) {
	assetName := currentAssetName("2.0.0")
	archive := makeTarGz(t, map[string]string{platformBinaryName(): fakeBinary})
	checksums := fmt.Sprintf("%s  %s\n", sha256Hex([]byte("tampered")), assetName)
	rel := serveRelease(t, assetName, archive, checksums)
	target := targetFile(t)

	err := applyUpdate(context.Background(), newHTTPClient(), rel, target)
	require.Error(t, err)
	assert.ErrorContains(t, err, "checksum mismatch")

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "old version", string(got), "binary must be left untouched")
}

func TestApplyUpdate_ChecksumMissing(t *testing.T) {
	assetName := currentAssetName("2.0.0")
	archive := makeTarGz(t, map[string]string{platformBinaryName(): fakeBinary})
	rel := serveRelease(t, assetName, archive, "deadbeef  some-other-file.tar.gz\n")
	target := targetFile(t)

	err := applyUpdate(context.Background(), newHTTPClient(), rel, target)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no checksum for")

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "old version", string(got))
}

func TestApplyUpdate_BinaryMissingFromArchive(t *testing.T) {
	assetName := currentAssetName("2.0.0")
	archive := makeTarGz(t, map[string]string{"README.md": "# docs"})
	checksums := fmt.Sprintf("%s  %s\n", sha256Hex(archive), assetName)
	rel := serveRelease(t, assetName, archive, checksums)
	target := targetFile(t)

	err := applyUpdate(context.Background(), newHTTPClient(), rel, target)
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found in archive")
}

func TestApplyUpdate_DownloadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	rel := &ReleaseInfo{
		AssetName:   currentAssetName("2.0.0"),
		AssetURL:    srv.URL + "/missing.tar.gz",
		ChecksumURL: srv.URL + "/checksums.txt",
	}

	err := applyUpdate(context.Background(), newHTTPClient(), rel, targetFile(t))
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to download")
}

func TestVerifyChecksum_IgnoresPartialNameMatch(t *testing.T) {
	archive := []byte("payload")
	checksums := fmt.Sprintf("%s  supermetrics-cli_2.0.0_linux_amd64.tar.gz.sig\n", sha256Hex(archive))

	err := verifyChecksum(archive, []byte(checksums), "supermetrics-cli_2.0.0_linux_amd64.tar.gz")
	require.Error(t, err)
	assert.ErrorContains(t, err, "no checksum for")
}
