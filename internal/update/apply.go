package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"runtime"
	"strings"

	"github.com/minio/selfupdate"
)

const binaryName = "supermetrics"

// applyUpdate downloads the release asset, verifies its sha256 against the
// release checksums file, and replaces the binary at execPath.
func applyUpdate(ctx context.Context, client *http.Client, rel *ReleaseInfo, execPath string) error {
	archive, err := download(ctx, client, rel.AssetURL)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", rel.AssetName, err)
	}

	checksums, err := download(ctx, client, rel.ChecksumURL)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", checksumAssetName, err)
	}

	if err := verifyChecksum(archive, checksums, rel.AssetName); err != nil {
		return err
	}

	binary, err := extractBinary(archive, rel.AssetName)
	if err != nil {
		return err
	}

	return selfupdate.Apply(bytes.NewReader(binary), selfupdate.Options{TargetPath: execPath})
}

func download(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
}

// verifyChecksum compares the asset's sha256 against its entry in a goreleaser
// checksums file ("<sha256>  <filename>" per line).
func verifyChecksum(archive, checksums []byte, assetName string) error {
	want := ""
	for line := range strings.Lines(string(checksums)) {
		sum, name, found := strings.Cut(strings.TrimSpace(line), "  ")
		if found && name == assetName {
			want = sum
			break
		}
	}
	if want == "" {
		return fmt.Errorf("%s has no checksum for %s", checksumAssetName, assetName)
	}

	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, want, got)
	}
	return nil
}

// extractBinary pulls the CLI binary out of a .tar.gz or .zip release archive,
// which also contains files such as README.md.
func extractBinary(archive []byte, assetName string) ([]byte, error) {
	binary := binaryName
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	var (
		data []byte
		err  error
	)
	if strings.HasSuffix(assetName, ".zip") {
		data, err = extractFromZip(archive, binary)
	} else {
		data, err = extractFromTarGz(archive, binary)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to extract %s from %s: %w", binary, assetName, err)
	}
	return data, nil
}

func extractFromTarGz(archive []byte, binary string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("not found in archive")
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeReg && path.Base(header.Name) == binary {
			return io.ReadAll(io.LimitReader(tr, maxResponseBytes))
		}
	}
}

func extractFromZip(archive []byte, binary string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}

	for _, file := range zr.File {
		if path.Base(file.Name) != binary {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()
		return io.ReadAll(io.LimitReader(rc, maxResponseBytes))
	}
	return nil, errors.New("not found in archive")
}
