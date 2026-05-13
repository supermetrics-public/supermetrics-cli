package generated

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMultipartTestCmd creates a minimal root+child cobra command with the flags
// that doRequest / printResult inspect. Mirrors the pattern in generated_test.go
// (TestExecuteRequestNoContent_Success).
func newMultipartTestCmd() (*cobra.Command, *cobra.Command) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("verbose", false, "")
	root.PersistentFlags().Bool("no-retry", false, "")
	root.PersistentFlags().Bool("quiet", false, "")
	root.PersistentFlags().Bool("no-color", false, "")
	root.PersistentFlags().String("timeout", "", "")
	root.PersistentFlags().String("output", "json", "")
	root.PersistentFlags().Bool("flatten", false, "")
	root.PersistentFlags().String("fields", "", "")

	cmd := &cobra.Command{Use: "test"}
	root.AddCommand(cmd)
	return root, cmd
}

// writeTempFile creates a temp file inside dir with the given content and returns its path.
func writeTempFile(t *testing.T, dir, content string) string {
	t.Helper()
	f, err := os.CreateTemp(dir, "upload-*.csv")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// parseMultipartBody reads all parts from a multipart body given the Content-Type header.
// Returns a map of field name → value for non-file parts and a separate map for file parts.
func parseMultipartBody(t *testing.T, body []byte, contentType string) (fields map[string]string, fileParts map[string]string) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(mediaType, "multipart/"), "expected multipart content type, got %s", mediaType)

	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	fields = make(map[string]string)
	fileParts = make(map[string]string)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		data, err := io.ReadAll(part)
		require.NoError(t, err)

		if part.FileName() != "" {
			fileParts[part.FormName()] = string(data)
		} else {
			fields[part.FormName()] = string(data)
		}
	}
	return fields, fileParts
}

// --- buildMultipartStream tests ---

func TestBuildMultipartStream_ValidFile(t *testing.T) {
	dir := t.TempDir()
	content := "col1,col2\nval1,val2\n"
	path := writeTempFile(t, dir, content)

	body, contentType, err := buildMultipartStream(path, "data_file", nil)
	require.NoError(t, err)
	require.NotNil(t, body)

	// Content-Type must be multipart/form-data with a boundary.
	assert.Contains(t, contentType, "multipart/form-data")
	assert.Contains(t, contentType, "boundary=")

	// Read and parse the body.
	raw, err := io.ReadAll(body)
	require.NoError(t, err)

	_, fileParts := parseMultipartBody(t, raw, contentType)

	// The file part must be present under the given field name.
	fileContent, ok := fileParts["data_file"]
	require.True(t, ok, "expected file part with field name 'data_file'")
	assert.Equal(t, content, fileContent)
}

func TestBuildMultipartStream_FilenameIsBasename(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "data")

	body, contentType, err := buildMultipartStream(path, "file", nil)
	require.NoError(t, err)

	raw, err := io.ReadAll(body)
	require.NoError(t, err)

	// Parse the raw multipart bytes to find the Content-Disposition filename.
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)

	mr := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
	found := false
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if part.FileName() != "" {
			assert.Equal(t, filepath.Base(path), part.FileName(), "filename in form should be the base name only")
			found = true
		}
	}
	assert.True(t, found, "expected to find at least one file part")
}

func TestBuildMultipartStream_WithFormFields(t *testing.T) {
	dir := t.TempDir()
	content := "hello"
	path := writeTempFile(t, dir, content)

	extraFields := map[string]string{
		"title":       "my upload",
		"description": "test desc",
	}

	body, contentType, err := buildMultipartStream(path, "payload", extraFields)
	require.NoError(t, err)

	raw, err := io.ReadAll(body)
	require.NoError(t, err)

	fields, fileParts := parseMultipartBody(t, raw, contentType)

	assert.Equal(t, "my upload", fields["title"], "title field should be present")
	assert.Equal(t, "test desc", fields["description"], "description field should be present")

	fileContent, ok := fileParts["payload"]
	require.True(t, ok, "expected file part 'payload'")
	assert.Equal(t, content, fileContent)
}

func TestBuildMultipartStream_NonexistentFile(t *testing.T) {
	_, _, err := buildMultipartStream("/nonexistent/path/does-not-exist.csv", "file", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")
}

// --- resolveFileInput tests ---

func TestResolveFileInput_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "content")

	resolved, cleanup, err := resolveFileInput(path)
	require.NoError(t, err)
	assert.Equal(t, path, resolved)

	// Cleanup for explicit paths is a no-op; the file must still exist afterwards.
	cleanup()
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "cleanup should not remove explicit file paths")
}

func TestResolveFileInput_EmptyPathTTY(t *testing.T) {
	// In the test environment stdin is not a TTY but we need to test the TTY branch.
	// We do this by temporarily replacing os.Stdin with the write-end of a pipe
	// (which reports as a terminal only on real terminals). In CI / tests stdin is
	// never a TTY, so the function will fall through to the temp-file path. To hit
	// the TTY error path reliably we cannot use a real terminal, so we rely on the
	// function's behaviour when passed an empty path with a non-pipe stdin.
	//
	// Instead, we test the documented contract: when stdin IS a terminal and filePath
	// is empty, an error is returned. We verify the error message text by reading the
	// source logic, which calls exitcode.Wrap with "provide a file path with --file
	// or pipe data via stdin". Because test stdin is not a TTY (it's a pipe supplied
	// by the test runner), this branch is not reachable from a normal test binary.
	// We document the untestable case and only verify the reachable branch below.
	t.Skip("TTY branch is only reachable from a real terminal; skipping in automated tests")
}

func TestResolveFileInput_EmptyPathStdinPipe(t *testing.T) {
	// Provide a readable pipe as stdin so resolveFileInput reads from it.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, _ = w.WriteString("piped data\n")
	w.Close()

	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })

	resolved, cleanup, err := resolveFileInput("")
	require.NoError(t, err)
	defer cleanup()

	// Resolved path must point to a temp file that exists.
	_, statErr := os.Stat(resolved)
	assert.NoError(t, statErr, "temp file should exist")

	// The file must contain what was piped.
	data, readErr := os.ReadFile(resolved)
	require.NoError(t, readErr)
	assert.Equal(t, "piped data\n", string(data))

	// After cleanup the temp file should be gone.
	cleanup()
	_, statErr = os.Stat(resolved)
	assert.True(t, os.IsNotExist(statErr), "cleanup should remove the temp file")
}

// --- executeMultipartRequest integration tests ---

func TestExecuteMultipartRequest_SendsCorrectContentType(t *testing.T) {
	dir := t.TempDir()
	fileContent := "id,name\n1,Alice\n"
	path := writeTempFile(t, dir, fileContent)

	var receivedContentType string
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")

		var readErr error
		receivedBody, readErr = io.ReadAll(r.Body)
		require.NoError(t, readErr)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"meta":{"request_id":"r1"},"data":{"status":"ok"}}`))
	}))
	defer srv.Close()

	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = nil })

	_, cmd := newMultipartTestCmd()

	body, contentType, err := buildMultipartStream(path, "upload", map[string]string{"note": "unit-test"})
	require.NoError(t, err)

	result, err := executeMultipartRequest(
		cmd,
		"POST",
		srv.URL+"/upload",
		body,
		contentType,
		"test-key",
		10*time.Second,
		"Uploading...",
	)
	require.NoError(t, err)

	// Verify the Content-Type header sent to the server.
	assert.Contains(t, receivedContentType, "multipart/form-data", "server should receive multipart/form-data content type")
	assert.Contains(t, receivedContentType, "boundary=", "content type should include boundary")

	// Verify the server received the correct multipart body.
	fields, fileParts := parseMultipartBody(t, receivedBody, receivedContentType)
	assert.Equal(t, "unit-test", fields["note"])
	assert.Equal(t, fileContent, fileParts["upload"])

	// Verify the parsed response.
	resultMap, ok := result.(map[string]any)
	require.True(t, ok, "result should be a map")
	assert.Equal(t, "ok", resultMap["status"])
}

func TestExecuteMultipartRequest_ReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"meta":{"request_id":"r2"},"error":{"code":"INVALID","message":"bad file"}}`))
	}))
	defer srv.Close()

	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = nil })

	dir := t.TempDir()
	path := writeTempFile(t, dir, "data")

	_, cmd := newMultipartTestCmd()

	body, contentType, err := buildMultipartStream(path, "f", nil)
	require.NoError(t, err)

	_, err = executeMultipartRequest(
		cmd,
		"POST",
		srv.URL+"/upload",
		body,
		contentType,
		"test-key",
		10*time.Second,
		"Uploading...",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad file")
}

func TestExecuteMultipartRequestNoContent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = nil })

	dir := t.TempDir()
	path := writeTempFile(t, dir, "payload")

	_, cmd := newMultipartTestCmd()

	body, contentType, err := buildMultipartStream(path, "file", nil)
	require.NoError(t, err)

	err = executeMultipartRequestNoContent(
		cmd,
		"POST",
		srv.URL+"/upload",
		body,
		contentType,
		"test-key",
		10*time.Second,
		"Uploading...",
	)
	assert.NoError(t, err)
}

func TestExecuteMultipartRequestNoContent_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"meta":{"request_id":"r3"},"error":{"code":"UNPROCESSABLE","message":"invalid format"}}`))
	}))
	defer srv.Close()

	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = nil })

	dir := t.TempDir()
	path := writeTempFile(t, dir, "payload")

	_, cmd := newMultipartTestCmd()

	body, contentType, err := buildMultipartStream(path, "file", nil)
	require.NoError(t, err)

	err = executeMultipartRequestNoContent(
		cmd,
		"POST",
		srv.URL+"/upload",
		body,
		contentType,
		"test-key",
		10*time.Second,
		"Uploading...",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid format")
}

func TestExecuteMultipartRequest_ResponseParsedAsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"meta":{"request_id":"r4"},"data":{"id":"upload-42","size":1024}}`))
	}))
	defer srv.Close()

	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = nil })

	dir := t.TempDir()
	path := writeTempFile(t, dir, "content")

	_, cmd := newMultipartTestCmd()

	body, contentType, err := buildMultipartStream(path, "file", nil)
	require.NoError(t, err)

	result, err := executeMultipartRequest(
		cmd,
		"POST",
		srv.URL+"/upload",
		body,
		contentType,
		"api-key",
		10*time.Second,
		"Uploading...",
	)
	require.NoError(t, err)

	resultMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upload-42", resultMap["id"])
	assert.Equal(t, float64(1024), resultMap["size"])
}
