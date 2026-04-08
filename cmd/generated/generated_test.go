package generated

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/supermetrics-public/supermetrics-cli/internal/auth"
)

// newTestRootCommand creates a root cobra command with the standard persistent flags
func newTestRootCommand() *cobra.Command {
	apiKey := ""
	output := "json"
	verbose := false
	noColor := false
	flatten := false
	noRetry := false
	quiet := false

	rootCmd := &cobra.Command{
		Use: "supermetrics",
	}
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "json", "Output format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color")
	rootCmd.PersistentFlags().BoolVar(&flatten, "flatten", false, "Flatten output")
	rootCmd.PersistentFlags().BoolVar(&noRetry, "no-retry", false, "Disable retry")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet")
	rootCmd.PersistentFlags().String("fields", "", "Fields to include in output")
	rootCmd.PersistentFlags().String("profile", "", "Named profile")
	rootCmd.PersistentFlags().String("timeout", "", "Override request timeout")

	RegisterAll(rootCmd)

	return rootCmd
}

// testServerTransport rewrites all request URLs to point at the test server.
type testServerTransport struct {
	serverURL string
}

func (t *testServerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point at the test server
	req.URL.Scheme = "http"

	// Extract host:port from serverURL (remove http://)
	host := strings.TrimPrefix(t.serverURL, "http://")
	req.URL.Host = host
	req.Header.Set("Host", host)
	req.RequestURI = ""

	return http.DefaultTransport.RoundTrip(req)
}

// setTestHTTPClient sets the httpClient to use a test server.
// Returns a cleanup function.
func setTestHTTPClient(srv *httptest.Server) func() {
	httpClient = &http.Client{
		Transport: &testServerTransport{serverURL: srv.URL},
	}
	return func() { httpClient = nil }
}

// setupOAuthEnv configures OAuth environment variables, viper, and resets caches.
func setupOAuthEnv(t *testing.T) {
	// Configure viper to read from environment
	viper.SetEnvPrefix("SUPERMETRICS")
	viper.AutomaticEnv()

	// Set values in viper directly
	viper.Set("oauth_client_id", "test-client-id")
	viper.Set("oauth_scopes", "openid")

	// Also set env vars for completeness
	oldClientID := os.Getenv("SUPERMETRICS_OAUTH_CLIENT_ID")
	oldScopes := os.Getenv("SUPERMETRICS_OAUTH_SCOPES")

	os.Setenv("SUPERMETRICS_OAUTH_CLIENT_ID", "test-client-id")
	os.Setenv("SUPERMETRICS_OAUTH_SCOPES", "openid")

	// Reset cache to pick up new values
	auth.ResetOAuthConfigCache()

	// Cleanup function to restore old values
	t.Cleanup(func() {
		viper.Set("oauth_client_id", "")
		viper.Set("oauth_scopes", "")
		if oldClientID == "" {
			os.Unsetenv("SUPERMETRICS_OAUTH_CLIENT_ID")
		} else {
			os.Setenv("SUPERMETRICS_OAUTH_CLIENT_ID", oldClientID)
		}
		if oldScopes == "" {
			os.Unsetenv("SUPERMETRICS_OAUTH_SCOPES")
		} else {
			os.Setenv("SUPERMETRICS_OAUTH_SCOPES", oldScopes)
		}
		auth.ResetOAuthConfigCache()
	})
}

// executeRootCommand executes the root command with given args.
// Assumes OAuth env vars are properly configured.
//
// Due to a Cobra limitation, shared package-level command variables
// (e.g. LoginsListCmd) retain parent pointers from previous Execute() calls.
// To work around this, we set the API key via environment variable instead
// of relying on --api-key flag parsing for most tests.
func executeRootCommand(args []string) (string, string, error) {
	// Extract --api-key from args and set via env to avoid Cobra re-parenting bug.
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--api-key" && i+1 < len(args) {
			os.Setenv("SUPERMETRICS_API_KEY", args[i+1])
			i++ // skip value
			continue
		}
		filteredArgs = append(filteredArgs, args[i])
	}

	rootCmd := newTestRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(filteredArgs)

	err := rootCmd.Execute()

	// Clean up env var.
	os.Unsetenv("SUPERMETRICS_API_KEY")

	return stdout.String(), stderr.String(), err
}

// TestAccountsListSuccess tests successful accounts list
func TestAccountsListSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v2/query/accounts")
		assert.Contains(t, r.URL.RawQuery, "ds_id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-123"},
			"data": {"accounts": [{"id": "1", "name": "Account 1"}]}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{"accounts", "list", "--api-key", "test-key", "--output", "json", "--ds-id", "ga"})
	require.NoError(t, err)
}

// TestAccountsListMissingRequiredFlag tests that ds-id flag is required
func TestAccountsListMissingRequiredFlag(t *testing.T) {

	_, _, err := executeRootCommand([]string{"accounts", "list", "--api-key", "test-key"})
	require.Error(t, err)
}

// TestQueriesExecuteSuccess tests successful query execution
func TestQueriesExecuteSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v2/query/data/json")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-456", "schedule_id": "sched-456", "status_code": "SUCCESS"},
			"data": {"rows": [[1, 2, 3]]}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--output", "json",
		"--ds-id", "ga",
		"--start-date", "2024-01-01",
		"--end-date", "2024-01-31",
	})
	require.NoError(t, err)
}

// TestQueriesExecuteMissingDsId tests that ds-id flag is required
func TestQueriesExecuteMissingDsId(t *testing.T) {

	_, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
	})
	require.Error(t, err)
}

// TestAsyncQueryImmediateCompletion tests that a fast query completes on first request
func TestAsyncQueryImmediateCompletion(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-imm", "schedule_id": "sched-imm", "status_code": "SUCCESS"},
			"data": [["a", "b"], ["c", "d"]]
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	stdout, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--output", "json",
		"--ds-id", "ga",
		"--start-date", "2024-01-01",
		"--end-date", "2024-01-31",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, `"a"`)
	assert.Contains(t, stdout, `"d"`)
}

// TestAsyncQueryPollSuccess tests that pending queries are polled until completion
func TestAsyncQueryPollSuccess(t *testing.T) {
	setupOAuthEnv(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if callCount == 1 {
			// First call: return scheduled (non-terminal)
			w.Write([]byte(`{
				"meta": {"request_id": "req-poll", "schedule_id": "sched-poll", "status_code": "SCHEDULED"},
				"data": null
			}`))
		} else {
			// Second call (poll): return completed
			// Verify poll uses schedule_id and includes ds_id
			jsonParam := r.URL.Query().Get("json")
			assert.Contains(t, jsonParam, "sched-poll")
			assert.Contains(t, jsonParam, "ga")
			w.Write([]byte(`{
				"meta": {"request_id": "req-poll", "schedule_id": "sched-poll", "status_code": "SUCCESS"},
				"data": [["x", "y"]]
			}`))
		}
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	stdout, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--output", "json",
		"--ds-id", "ga",
		"--start-date", "2024-01-01",
		"--end-date", "2024-01-31",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, `"x"`)
	assert.GreaterOrEqual(t, callCount, 2)
}

// TestAsyncQueryInitialError tests that API errors on initial request are returned
func TestAsyncQueryInitialError(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-err"},
			"error": {"code": "INVALID", "message": "Bad parameter"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--ds-id", "ga",
		"--start-date", "2024-01-01",
		"--end-date", "2024-01-31",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bad parameter")
}

// TestAsyncQueryFailureStatus tests that FAILURE status returns an error
func TestAsyncQueryFailureStatus(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-fail", "schedule_id": "sched-fail", "status_code": "FAILURE"},
			"data": null
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--ds-id", "ga",
		"--start-date", "2024-01-01",
		"--end-date", "2024-01-31",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FAILURE")
}

// TestAsyncQuerySendsZeroSyncTimeout verifies sync_timeout=0 is sent in the request
func TestAsyncQuerySendsZeroSyncTimeout(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonParam := r.URL.Query().Get("json")
		assert.Contains(t, jsonParam, `"sync_timeout":0`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-sync", "schedule_id": "sched-sync", "status_code": "SUCCESS"},
			"data": []
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--output", "json",
		"--ds-id", "ga",
		"--start-date", "2024-01-01",
		"--end-date", "2024-01-31",
	})
	require.NoError(t, err)
}

// --- Pagination tests ---

// resetPaginationFlags resets the package-level pagination flag variables
// that persist between tests due to Cobra's binding to shared command objects.
func resetPaginationFlags(t *testing.T) {
	t.Helper()
	flagQueriesExecuteAll = false
	flagQueriesExecuteLimit = 0
	t.Cleanup(func() {
		flagQueriesExecuteAll = false
		flagQueriesExecuteLimit = 0
	})
}

func TestQueriesExecutePaginationAll(t *testing.T) {
	setupOAuthEnv(t)
	resetPaginationFlags(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if callCount == 1 {
			// First call: async query returns SUCCESS with page 1
			w.Write([]byte(`{
				"meta": {
					"request_id": "req-1", "schedule_id": "sched-1", "status_code": "SUCCESS",
					"result": {"total_rows": 5},
					"paginate": {"next": "` + "https://api.example.com/v2/query/data/json?json=page2" + `"}
				},
				"data": [["id","name"],["1","Alice"],["2","Bob"],["3","Charlie"]]
			}`))
		} else {
			// Second call: page 2, no more pages
			w.Write([]byte(`{
				"meta": {
					"request_id": "req-2", "status_code": "SUCCESS",
					"result": {"total_rows": 5},
					"paginate": {}
				},
				"data": [["id","name"],["4","Dave"],["5","Eve"]]
			}`))
		}
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	stdout, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--output", "json",
		"--ds-id", "ga",
		"--all",
	})
	require.NoError(t, err)

	// Should have fetched both pages
	assert.Equal(t, 2, callCount, "should have made 2 requests (initial + page 2)")

	// Parse output: should be header + 5 data rows
	var result []any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Len(t, result, 6, "expected 1 header row + 5 data rows")
}

func TestQueriesExecutePaginationLimit(t *testing.T) {
	setupOAuthEnv(t)
	resetPaginationFlags(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return 3 data rows with pagination available
		w.Write([]byte(`{
			"meta": {
				"request_id": "req-1", "schedule_id": "sched-1", "status_code": "SUCCESS",
				"result": {"total_rows": 10},
				"paginate": {"next": "https://api.example.com/v2/query/data/json?json=page2"}
			},
			"data": [["id","name"],["1","A"],["2","B"],["3","C"]]
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	stdout, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--output", "json",
		"--ds-id", "ga",
		"--limit", "2",
	})
	require.NoError(t, err)

	// Should only fetch first page since limit (2) < page size (3)
	assert.Equal(t, 1, callCount, "should not fetch page 2 when limit is satisfied by page 1")

	// Parse output: should be header + 2 data rows (limited)
	var result []any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Len(t, result, 3, "expected 1 header row + 2 data rows (limited)")
}

func TestQueriesExecuteNoPaginationFlag(t *testing.T) {
	setupOAuthEnv(t)
	resetPaginationFlags(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {
				"request_id": "req-1", "schedule_id": "sched-1", "status_code": "SUCCESS",
				"result": {"total_rows": 10},
				"paginate": {"next": "https://api.example.com/v2/query/data/json?json=page2"}
			},
			"data": [["id","name"],["1","A"],["2","B"],["3","C"]]
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	stdout, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--output", "json",
		"--ds-id", "ga",
	})
	require.NoError(t, err)

	// Without --all or --limit, should NOT follow pagination
	assert.Equal(t, 1, callCount, "should not follow pagination without --all or --limit")

	// Parse output: should be header + 3 data rows (single page only)
	var result []any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Len(t, result, 4, "expected 1 header row + 3 data rows (no pagination)")
}

// TestBackfillsCreateSuccess tests successful backfill creation
func TestBackfillsCreateSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/teams/123/transfers/456/backfills")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-789"},
			"data": {"id": 1, "status": "PENDING"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--output", "json",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-start", "2024-01-01",
		"--range-end", "2024-01-31",
	})
	require.NoError(t, err)
}

// TestBackfillsCreateMissingRangeStart tests that range-start is required
func TestBackfillsCreateMissingRangeStart(t *testing.T) {

	_, _, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-end", "2024-01-31",
	})
	require.Error(t, err)
}

// TestBackfillsCreateMissingRangeEnd tests that range-end is required
func TestBackfillsCreateMissingRangeEnd(t *testing.T) {

	_, _, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-start", "2024-01-01",
	})
	require.Error(t, err)
}

// TestBackfillsGetSuccess tests successful backfill get
func TestBackfillsGetSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/teams/123/backfills/789")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-abc"},
			"data": {"id": 789, "status": "COMPLETED"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "get",
		"--api-key", "test-key",
		"--team-id", "123",
		"--backfill-id", "789",
	})
	require.NoError(t, err)
}

// TestBackfillsGetLatestSuccess tests successful get latest backfill
func TestBackfillsGetLatestSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/teams/123/transfers/456/backfills/latest")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-def"},
			"data": {"id": 789, "status": "COMPLETED"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "get-latest",
		"--api-key", "test-key",
		"--team-id", "123",
		"--transfer-id", "456",
	})
	require.NoError(t, err)
}

// TestBackfillsListIncompleteSuccess tests successful list incomplete
func TestBackfillsListIncompleteSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/teams/123/backfills")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-ghi"},
			"data": [{"id": 1, "status": "PENDING"}]
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "list-incomplete",
		"--api-key", "test-key",
		"--team-id", "123",
	})
	require.NoError(t, err)
}

// TestBackfillsCancelSuccess tests successful backfill cancel
func TestBackfillsCancelSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/teams/123/backfills/789")
		// Verify request body contains status: CANCELLED
		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]string
		err := json.Unmarshal(body, &reqBody)
		require.NoError(t, err)
		assert.Equal(t, "CANCELLED", reqBody["status"])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-jkl"},
			"data": {"id": 789, "status": "CANCELLED"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "cancel",
		"--api-key", "test-key",
		"--team-id", "123",
		"--backfill-id", "789",
	})
	require.NoError(t, err)
}

// TestLoginLinksCreateSuccess tests successful login link creation
func TestLoginLinksCreateSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/v2/ds/login/link")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-mno"},
			"data": {"id": "link-123", "ds_id": "ga", "expiry_time": "2024-02-01"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"login-links", "create",
		"--api-key", "test-key",
		"--ds-id", "ga",
		"--expiry-time", "2024-02-01",
	})
	require.NoError(t, err)
}

// TestLoginLinksCreateMissingDsId tests that ds-id is required
func TestLoginLinksCreateMissingDsId(t *testing.T) {

	_, _, err := executeRootCommand([]string{
		"login-links", "create",
		"--api-key", "test-key",
		"--expiry-time", "2024-02-01",
	})
	require.Error(t, err)
}

// TestLoginLinksCreateMissingExpiryTime tests that expiry-time is required
func TestLoginLinksCreateMissingExpiryTime(t *testing.T) {

	_, _, err := executeRootCommand([]string{
		"login-links", "create",
		"--api-key", "test-key",
		"--ds-id", "ga",
	})
	require.Error(t, err)
}

// TestLoginLinksListSuccess tests successful login links list
func TestLoginLinksListSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v2/ds/login/links")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-pqr"},
			"data": [{"id": "link-123", "ds_id": "ga"}]
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"login-links", "list",
		"--api-key", "test-key",
	})
	require.NoError(t, err)
}

// TestLoginLinksCloseSuccess tests successful login link close
func TestLoginLinksCloseSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/v2/ds/login/link/{link_id}/close")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-stu"},
			"data": {"id": "link-123", "status": "CLOSED"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"login-links", "close",
		"--api-key", "test-key",
	})
	require.NoError(t, err)
}

// TestLoginsGetSuccess tests successful logins get
func TestLoginsGetSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v2/ds/login/{login_id}")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-vwx"},
			"data": {"id": "login-123", "ds_id": "ga"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "get",
		"--api-key", "test-key",
	})
	require.NoError(t, err)
}

// TestLoginsListSuccess tests successful logins list
func TestLoginsListSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v2/ds/logins")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-yza"},
			"data": [{"id": "login-123", "ds_id": "ga"}]
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "list",
		"--api-key", "test-key",
	})
	require.NoError(t, err)
}

// TestDatasourceGetSuccess tests successful datasource get
func TestDatasourceGetSuccess(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/teams/123/datasource/ga")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-bcd"},
			"data": {"id": "ga", "name": "Google Analytics"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"datasource", "get",
		"--api-key", "test-key",
		"--team-id", "123",
		"--data-source-id", "ga",
	})
	require.NoError(t, err)
}

// TestDatasourceGetMissingTeamId tests that team-id is required
func TestDatasourceGetMissingTeamId(t *testing.T) {

	_, _, err := executeRootCommand([]string{
		"datasource", "get",
		"--api-key", "test-key",
		"--data-source-id", "ga",
	})
	require.Error(t, err)
}

// TestDatasourceGetMissingDataSourceId tests that data-source-id is required
func TestDatasourceGetMissingDataSourceId(t *testing.T) {

	_, _, err := executeRootCommand([]string{
		"datasource", "get",
		"--api-key", "test-key",
		"--team-id", "123",
	})
	require.Error(t, err)
}

// TestAuthenticationError tests 401 authentication error
func TestAuthenticationError(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{
			"meta": {"request_id": "req-err1"},
			"error": {"code": "unauthorized", "message": "Invalid API key"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "list",
		"--api-key", "bad-key",
	})
	// Error is expected
	require.Error(t, err)
}

// TestAPIError tests API error response with error envelope
func TestAPIError(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{
			"meta": {"request_id": "req-err2"},
			"error": {"code": "invalid_params", "message": "Invalid parameters"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"accounts", "list",
		"--api-key", "test-key",
		"--ds-id", "invalid",
	})
	// Error is expected
	require.Error(t, err)
}

// TestOutputFormatJSON tests JSON output format
func TestOutputFormatJSON(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-out1"},
			"data": {"accounts": [{"id": "1", "name": "Account 1"}]}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"accounts", "list",
		"--api-key", "test-key",
		"--output", "json",
		"--ds-id", "ga",
	})
	require.NoError(t, err)
}

// TestOutputFormatTable tests table output format
func TestOutputFormatTable(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-out2"},
			"data": [{"id": "1", "name": "Account 1"}, {"id": "2", "name": "Account 2"}]
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "list",
		"--api-key", "test-key",
		"--output", "table",
	})
	require.NoError(t, err)
}

// TestOutputFormatCSV tests CSV output format
func TestOutputFormatCSV(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-out3"},
			"data": [{"id": "1", "name": "Account 1"}]
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "list",
		"--api-key", "test-key",
		"--output", "csv",
	})
	require.NoError(t, err)
}

// TestCleanZeroValuesString tests cleanZeroValues removes empty strings
func TestCleanZeroValuesString(t *testing.T) {
	params := map[string]any{
		"name":  "",
		"value": "test",
	}
	cleanZeroValues(params)
	_, ok := params["name"]
	assert.False(t, ok, "empty string should be removed")
	assert.Equal(t, "test", params["value"], "non-empty string should be kept")
}

// TestCleanZeroValuesInt tests cleanZeroValues removes zero integers
func TestCleanZeroValuesInt(t *testing.T) {
	params := map[string]any{
		"count":  0,
		"offset": 5,
	}
	cleanZeroValues(params)
	_, ok := params["count"]
	assert.False(t, ok, "zero int should be removed")
	assert.Equal(t, 5, params["offset"], "non-zero int should be kept")
}

// TestCleanZeroValuesInt64 tests cleanZeroValues removes zero int64
func TestCleanZeroValuesInt64(t *testing.T) {
	params := map[string]any{
		"team_id": int64(0),
		"user_id": int64(123),
	}
	cleanZeroValues(params)
	_, ok := params["team_id"]
	assert.False(t, ok, "zero int64 should be removed")
	assert.Equal(t, int64(123), params["user_id"], "non-zero int64 should be kept")
}

// TestCleanZeroValuesFloat64 tests cleanZeroValues removes zero float64
func TestCleanZeroValuesFloat64(t *testing.T) {
	params := map[string]any{
		"rate":  0.0,
		"ratio": 0.5,
	}
	cleanZeroValues(params)
	_, ok := params["rate"]
	assert.False(t, ok, "zero float64 should be removed")
	assert.Equal(t, 0.5, params["ratio"], "non-zero float64 should be kept")
}

// TestCleanZeroValuesSlice tests cleanZeroValues removes empty slices
func TestCleanZeroValuesSlice(t *testing.T) {
	params := map[string]any{
		"tags":  []string{},
		"items": []string{"a", "b"},
	}
	cleanZeroValues(params)
	_, ok := params["tags"]
	assert.False(t, ok, "empty slice should be removed")
	assert.NotNil(t, params["items"], "non-empty slice should be kept")
}

// TestResolveAuthMissingCredentials tests that resolveAuth returns error when no credentials
func TestResolveAuthMissingCredentials(t *testing.T) {
	// Set OAuth config first, then clear API key to test credential resolution failure
	setupOAuthEnv(t)
	os.Setenv("SUPERMETRICS_API_KEY", "")
	t.Cleanup(func() { os.Unsetenv("SUPERMETRICS_API_KEY") })

	_, _, err := executeRootCommand([]string{"logins", "list"})
	require.Error(t, err)
}

// TestResolveAuthFromEnv tests that resolveAuth uses API key from env/flag
func TestResolveAuthFromEnv(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Contains(t, authHeader, "flag-key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"meta": {"request_id": "req-ok"}, "data": []}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "list",
		"--api-key", "flag-key",
	})
	require.NoError(t, err)
}

// TestGetDomain tests domain resolution
func TestGetDomain(t *testing.T) {
	domain := GetDomain()
	assert.NotEmpty(t, domain, "domain should not be empty")
}

// TestQueryParameterEncoding tests that query parameters are properly encoded
func TestQueryParameterEncoding(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "json=")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"meta": {"request_id": "req-enc"}, "data": {}}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"accounts", "list",
		"--api-key", "test-key",
		"--ds-id", "test-ds",
	})
	require.NoError(t, err)
}

// TestMultipleStringFlags tests that string slice flags work correctly
func TestMultipleStringFlags(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"meta": {"request_id": "req-multi", "schedule_id": "sched-multi", "status_code": "SUCCESS"}, "data": {}}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"queries", "execute",
		"--api-key", "test-key",
		"--ds-id", "ga",
		"--ds-users", "user1",
		"--ds-users", "user2",
	})
	require.NoError(t, err)
}

// TestVerboseLogging tests verbose flag doesn't cause errors
func TestVerboseLogging(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"meta": {"request_id": "req-verb"}, "data": []}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "list",
		"--api-key", "test-key",
		"--verbose",
	})
	require.NoError(t, err)
}

// TestNoRetryFlag tests that no-retry flag is passed through
func TestNoRetryFlag(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"meta": {"request_id": "req-noretry"}, "data": []}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "list",
		"--api-key", "test-key",
		"--no-retry",
	})
	require.NoError(t, err)
}

// TestNoColorFlag tests that no-color flag doesn't cause errors
func TestNoColorFlag(t *testing.T) {
	setupOAuthEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"meta": {"request_id": "req-nocol"}, "data": []}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"logins", "list",
		"--api-key", "test-key",
		"--no-color",
	})
	require.NoError(t, err)
}

// TestBackfillsCreateDryRun tests that --dry-run prints request details without executing
func TestBackfillsCreateDryRun(t *testing.T) {
	setupOAuthEnv(t)

	// No test server — dry-run should NOT make any HTTP call
	stdout, stderr, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-start", "2024-01-01",
		"--range-end", "2024-01-31",
		"--dry-run",
	})
	require.NoError(t, err)

	// stdout should be empty (no result printed)
	assert.Empty(t, stdout, "expected empty stdout")

	// stderr should contain the method and URL
	assert.Contains(t, stderr, "POST", "expected POST in stderr")
	assert.Contains(t, stderr, "/v1/teams/123/transfers/456/backfills", "expected backfills URL in stderr")

	// stderr should contain the body
	assert.Contains(t, stderr, "range_start", "expected range_start in body output")
}

// TestBackfillsCancelDryRun tests that --dry-run prints request details for cancel
func TestBackfillsCancelDryRun(t *testing.T) {
	setupOAuthEnv(t)

	stdout, stderr, err := executeRootCommand([]string{
		"backfills", "cancel",
		"--api-key", "test-key",
		"--team-id", "123",
		"--backfill-id", "789",
		"--dry-run",
	})
	require.NoError(t, err)

	assert.Empty(t, stdout, "expected empty stdout")
	assert.Contains(t, stderr, "PATCH", "expected PATCH in stderr")
	assert.Contains(t, stderr, "CANCELLED", "expected CANCELLED in body output")
}

// TestBackfillsCancelYesFlag tests that --yes skips confirmation
func TestBackfillsCancelYesFlag(t *testing.T) {
	setupOAuthEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req-yes"},
			"data": {"id": 789, "status": "CANCELLED"}
		}`))
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "cancel",
		"--api-key", "test-key",
		"--team-id", "123",
		"--backfill-id", "789",
		"--yes",
	})
	require.NoError(t, err)
}

// TestConfirmActionNonTTY tests that confirmAction skips prompt for non-TTY stdin
func TestConfirmActionNonTTY(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation")

	// In tests, stdin is not a TTY, so confirmation should be skipped
	err := confirmAction(cmd, "Are you sure?")
	require.NoError(t, err)
}

// setupBackfillWaitTest sets a fast poll interval and resets leaked flag state on BackfillsCreateCmd.
// Cobra doesn't reset flag values between Execute() calls on reused command instances,
// so bool flags like --dry-run set by earlier tests persist.
func setupBackfillWaitTest(t *testing.T) {
	t.Helper()
	backfillPollInterval = 1 * time.Millisecond
	_ = BackfillsCreateCmd.Flags().Set("dry-run", "false")
	_ = BackfillsCreateCmd.Flags().Set("wait", "false")
	t.Cleanup(func() {
		backfillPollInterval = 5 * time.Second
		_ = BackfillsCreateCmd.Flags().Set("dry-run", "false")
		_ = BackfillsCreateCmd.Flags().Set("wait", "false")
	})
}

// TestBackfillsCreateWaitSuccess tests --wait polls until COMPLETED
func TestBackfillsCreateWaitSuccess(t *testing.T) {
	setupOAuthEnv(t)
	setupBackfillWaitTest(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if callCount == 1 {
			// Create request
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/v1/teams/123/transfers/456/backfills")
			w.Write([]byte(`{
				"meta": {"request_id": "req-create"},
				"data": {"transfer_backfill_id": 999, "status": "CREATED", "transfer_runs_total": 10, "transfer_runs_completed": 0, "transfer_runs_failed": 0}
			}`))
		} else {
			// Poll request
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/v1/teams/123/backfills/999")
			w.Write([]byte(`{
				"meta": {"request_id": "req-poll"},
				"data": {"transfer_backfill_id": 999, "status": "COMPLETED", "transfer_runs_total": 10, "transfer_runs_completed": 10, "transfer_runs_failed": 0}
			}`))
		}
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	stdout, _, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--output", "json",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-start", "2024-01-01",
		"--range-end", "2024-01-31",
		"--wait",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 2, "expected at least 2 calls (create + poll)")
	assert.Contains(t, stdout, "COMPLETED")
}

// TestBackfillsCreateWaitPolling tests --wait polls multiple times before completion
func TestBackfillsCreateWaitPolling(t *testing.T) {
	setupOAuthEnv(t)
	setupBackfillWaitTest(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch callCount {
		case 1:
			w.Write([]byte(`{
				"meta": {"request_id": "req-create"},
				"data": {"transfer_backfill_id": 100, "status": "CREATED", "transfer_runs_total": 5, "transfer_runs_completed": 0, "transfer_runs_failed": 0}
			}`))
		case 2:
			w.Write([]byte(`{
				"meta": {"request_id": "req-poll1"},
				"data": {"transfer_backfill_id": 100, "status": "RUNNING", "transfer_runs_total": 5, "transfer_runs_completed": 3, "transfer_runs_failed": 0}
			}`))
		default:
			w.Write([]byte(`{
				"meta": {"request_id": "req-poll2"},
				"data": {"transfer_backfill_id": 100, "status": "COMPLETED", "transfer_runs_total": 5, "transfer_runs_completed": 5, "transfer_runs_failed": 0}
			}`))
		}
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--output", "json",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-start", "2024-01-01",
		"--range-end", "2024-01-31",
		"--wait",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 3, "expected at least 3 calls (create + 2 polls)")
}

// TestBackfillsCreateWaitFailure tests --wait returns error on FAILED status
func TestBackfillsCreateWaitFailure(t *testing.T) {
	setupOAuthEnv(t)
	setupBackfillWaitTest(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if callCount == 1 {
			w.Write([]byte(`{
				"meta": {"request_id": "req-create"},
				"data": {"transfer_backfill_id": 200, "status": "CREATED", "transfer_runs_total": 5, "transfer_runs_completed": 0, "transfer_runs_failed": 0}
			}`))
		} else {
			w.Write([]byte(`{
				"meta": {"request_id": "req-poll"},
				"data": {"transfer_backfill_id": 200, "status": "FAILED", "transfer_runs_total": 5, "transfer_runs_completed": 2, "transfer_runs_failed": 3}
			}`))
		}
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--output", "json",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-start", "2024-01-01",
		"--range-end", "2024-01-31",
		"--wait",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}

// TestBackfillsCreateWaitCancelled tests --wait returns error on CANCELLED status
func TestBackfillsCreateWaitCancelled(t *testing.T) {
	setupOAuthEnv(t)
	setupBackfillWaitTest(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if callCount == 1 {
			w.Write([]byte(`{
				"meta": {"request_id": "req-create"},
				"data": {"transfer_backfill_id": 300, "status": "CREATED", "transfer_runs_total": 5, "transfer_runs_completed": 0, "transfer_runs_failed": 0}
			}`))
		} else {
			w.Write([]byte(`{
				"meta": {"request_id": "req-poll"},
				"data": {"transfer_backfill_id": 300, "status": "CANCELLED", "transfer_runs_total": 5, "transfer_runs_completed": 1, "transfer_runs_failed": 0}
			}`))
		}
	}))
	defer srv.Close()
	defer setTestHTTPClient(srv)()

	_, _, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--output", "json",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-start", "2024-01-01",
		"--range-end", "2024-01-31",
		"--wait",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// TestBackfillsCreateDryRunSkipsWait tests that --dry-run prevents --wait from executing
func TestBackfillsCreateDryRunSkipsWait(t *testing.T) {
	setupOAuthEnv(t)

	// No test server — dry-run should NOT make any HTTP call
	stdout, stderr, err := executeRootCommand([]string{
		"backfills", "create",
		"--api-key", "test-key",
		"--team-id", "123",
		"--transfer-id", "456",
		"--range-start", "2024-01-01",
		"--range-end", "2024-01-31",
		"--dry-run",
		"--wait",
	})
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "POST")
}

// TestIsBackfillDone tests terminal state detection
func TestIsBackfillDone(t *testing.T) {
	assert.True(t, isBackfillDone("COMPLETED"))
	assert.True(t, isBackfillDone("FAILED"))
	assert.True(t, isBackfillDone("CANCELLED"))
	assert.False(t, isBackfillDone("CREATED"))
	assert.False(t, isBackfillDone("SCHEDULED"))
	assert.False(t, isBackfillDone("RUNNING"))
	assert.False(t, isBackfillDone(""))
}

// TestToFloat64 tests safe numeric conversion
func TestToFloat64(t *testing.T) {
	assert.Equal(t, 42.0, toFloat64(float64(42)))
	assert.Equal(t, 0.0, toFloat64("not a number"))
	assert.Equal(t, 0.0, toFloat64(nil))
	assert.Equal(t, 0.0, toFloat64(42)) // int, not float64
}

// TestDryRunRequestOutput tests that dryRunRequest prints method and URL
func TestDryRunRequestOutput(t *testing.T) {
	cmd := &cobra.Command{}
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)

	dryRunRequest(cmd, "DELETE", "https://api.example.com/v1/resource/123", strings.NewReader(`{"key":"value"}`))

	output := errBuf.String()

	assert.Contains(t, output, "DELETE", "expected DELETE in output")
	assert.Contains(t, output, "https://api.example.com/v1/resource/123", "expected URL in output")
	assert.Contains(t, output, `"key":"value"`, "expected body in output")
}

func TestResolveTimeout(t *testing.T) {
	tests := []struct {
		name       string
		flagValue  string
		defaultVal time.Duration
		expected   time.Duration
	}{
		{"flag 10s", "10s", 30 * time.Second, 10 * time.Second},
		{"flag 2m", "2m", 30 * time.Second, 2 * time.Minute},
		{"flag 1h", "1h", 30 * time.Second, 1 * time.Hour},
		{"no flag uses default", "", 30 * time.Second, 30 * time.Second},
		{"invalid value uses default", "banana", 30 * time.Second, 30 * time.Second},
		{"no flag uses long default", "", 60 * time.Minute, 60 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			root := &cobra.Command{Use: "root"}
			root.PersistentFlags().String("timeout", "", "Override timeout")
			root.AddCommand(cmd)

			if tt.flagValue != "" {
				require.NoError(t, root.PersistentFlags().Set("timeout", tt.flagValue))
			}

			got := resolveTimeout(cmd, tt.defaultVal)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResolveTimeout_NoFlag(t *testing.T) {
	// When root command has no --timeout flag at all (nil Lookup), returns default
	cmd := &cobra.Command{Use: "test"}
	root := &cobra.Command{Use: "root"}
	root.AddCommand(cmd)

	got := resolveTimeout(cmd, 42*time.Second)
	assert.Equal(t, 42*time.Second, got)
}
