# CLAUDE.md

## Project overview

Supermetrics CLI (`supermetrics`) — a Go CLI for the Supermetrics API. Ships as a single binary with no runtime
dependencies. Commands are auto-generated from an OpenAPI spec.

## Build and run

```bash
cp .env.example .env  # One-time: set up development environment variables
make tools            # One-time: install goimports, govulncheck, gotestsum, golangci-lint, ruff
make build            # Build to bin/supermetrics
make run ARGS="queries execute --help"  # Quick run without building
```

## Test and lint

```bash
make test           # All tests (Go + Python generator)
make test-go        # Go tests only
make test-python    # Python generator tests only (via uv)
make test-coverage  # With race detector and coverage report
make vet            # Static analysis
make lint           # golangci-lint + ruff (Python scripts)
make lint-python    # Ruff only (Python scripts)
make lint-fix       # Auto-fix lint and formatting issues (Go + Python)
make vulncheck      # Known vulnerability scan
```

## Code generation

CLI commands are generated from `openapi-spec.yaml` + `scripts/command-mapping.yaml`. **Never edit files in
`cmd/generated/` by hand** — they are overwritten by `make generate`.

To add a new CLI command: add an entry to `scripts/command-mapping.yaml` and run `make generate`.

### Generator feature flags in command-mapping.yaml

Per-command features controlled via YAML fields in `scripts/command-mapping.yaml`:

- `dry_run: true` — `--dry-run` flag prints request (method, URL, body) without executing. Used by `login-links create`,
  `backfills create`, `backfills cancel`
- `confirm: "message"` — `-y, --yes` flag skips interactive confirmation. Supports `{field}` substitution. Used by
  `login-links close`, `backfills cancel`
- `fixed_values: {field: value}` — values merged into request body, not exposed as flags. Example: `backfills cancel`
  sets `status: "CANCELLED"`
- `exclude_params: [...]` — hide OpenAPI params from CLI flags. Example: `queries execute` excludes `sync_timeout`
- `async: true` — async execution with adaptive polling (1s → 2s → 5s). Used by `queries execute`
- `wait: true` — `--wait` flag polls until completion with progress bar. Used by `backfills create`
- `spinner_text: "..."` — per-command spinner message on stderr (elapsed time auto-appended)
- `timeout: "60m"` — override default 30s timeout. Go duration format. Used by long-running commands
- `paginated: true` — `--all` and `--limit` flags for auto-pagination. Follows `meta.paginate.next` URLs, accumulates
  data rows across pages. Used by `queries execute`

The generator (`scripts/generate_commands.py`) produces Cobra commands that use the shared HTTP client at
`internal/httpclient/client.go` for all API calls. The generator runs via `uv run python3` and requires Python 3.13+
(pinned in `.python-version`). Python scripts are linted with ruff (config in `pyproject.toml`).

## Architecture

- `cmd/root.go` — root command, global flags, `.env` loading, config loading for defaults
- `cmd/generated/` — auto-generated resource commands (one file per resource group, plus `register.go` with shared
  helpers)
- `internal/httpclient/` — shared HTTP client: timeouts, auth headers, verbose logging (with request IDs), API response
  envelope unwrapping, centralized error handling
- `internal/auth/` — credential resolution (`--api-key` flag > env var > OAuth token with auto-refresh > API key from
  config) and OAuth PKCE login flow
- `internal/buildcfg/` — centralized build-time values injected via `-ldflags` (version, commit, OAuth client config)
- `internal/config/` — config file at `~/.config/supermetrics/config.json` (0600 permissions), validated before saving.
  Supports named profiles: `Config` has `Profiles map[string]*Profile` (credentials) and `ActiveProfile` (current
  selection). Global settings (`default_output`, update check) stay top-level. `Profile` holds `APIKey`, `AccessToken`,
  `RefreshToken`, `TokenExpiry`
- `internal/output/` — JSON/table/CSV formatting with box-drawing table borders, green headers, and cyan summaries for
  nested data. CSV always flattens nested data (row expansion for arrays of objects, dot-notation for nested objects,
  semicolons for primitive arrays). Table flattens only with `--flatten`. `NO_COLOR` / `--no-color` support.
  Client-side `--fields` filtering in `fields.go` with dot-notation (e.g. `id,error.message`), applied in `Print()`
  before format dispatch
- `internal/update/` — background update checking and self-update via GitHub Releases
- `internal/exitcode/` — BSD sysexits exit codes (Usage=64, Auth=65, Unavailable=69) with `Wrap`/`Of` helpers

## Key conventions

- All HTTP requests go through `internal/httpclient.Do()` — never use `http.DefaultClient` directly
- The Supermetrics API wraps responses in an envelope: `{meta: {request_id}, data: ...}` for success,
  `{meta: ..., error: {code, message, description}}` for errors. `Response.ParseJSON()` unwraps this automatically —
  callers get just the `data` payload. API-level errors (including HTTP 200 with `error` field) are returned as Go
  errors with the error code, message, and request ID
- Default request timeout is 30s. Long-running commands (queries, accounts) use 60m — configured via `timeout` field in
  `command-mapping.yaml`. Users can override per-command defaults with `--timeout` flag (Go duration format: `30s`, `5m`,
  `1h`). `resolveTimeout()` in generated `request.go` checks the flag and falls back to the per-command default
- Generated code imports are managed by `goimports` — the generator emits all potentially needed imports and `goimports`
  cleans up unused ones
- Auth priority: `--api-key` flag > `SUPERMETRICS_API_KEY` env > OAuth token (auto-refreshed) > API key from config.
  All config-based credentials are read from the resolved profile. OAuth uses Authorization Code + PKCE with a localhost
  callback. Client ID and scopes are injected at build time via `internal/buildcfg` (from `.env` locally, from CI
  secrets in releases)
- Profile priority: `--profile` flag > `SUPERMETRICS_PROFILE` env > `active_profile` in config > `"default"`.
  Profiles are created implicitly by `configure --profile <name>` or `login --profile <name>`. Managed via
  `supermetrics profile list|use|delete|show`. `resolveProfileName()` in generated `register.go` handles resolution
  for API commands; `GetProfile()` in `cmd/root.go` handles it for hand-written commands (`configure`, `login`,
  `logout`)
- Config priority: `--flag` > env var (`SUPERMETRICS_*`) > config file > hardcoded default (e.g.,
  `--output`/`SUPERMETRICS_OUTPUT`/config `default_output`). Domain uses a separate chain:
  `SUPERMETRICS_DOMAIN` env var > `buildcfg.DefaultDomain` (ldflags, defaults to `supermetrics.com`)
- Config values are validated before saving (see `Config.Validate()`)
- `--fields` is a global persistent flag (string, comma-separated) for client-side field filtering with dot-notation.
  `queries execute` has its own local `--fields` flag (StringSlice) for server-side field selection. Cobra's flag
  shadowing ensures the local flag takes precedence — when `queries execute --fields` is used, the global persistent
  `--fields` value stays empty and client-side filtering does not apply
- Color is disabled by `--no-color` flag, `NO_COLOR` env var, or non-TTY stdout
- Retry middleware: 3 attempts, exponential backoff with full jitter (500ms base, 20s max), `Retry-After` header
  support on 429. Disabled via `--no-retry` flag → `DisableRetry: true` in `httpclient.Do()` options
- Quiet mode via `--quiet`/`-q` flag or `SUPERMETRICS_QUIET` env var (non-empty, non-`"0"`). `infoWriter()`/
  `infoWriterErr()` return `io.Discard` in quiet mode — errors and results still printed normally
- Builds use `CGO_ENABLED=0` and `-trimpath` for static, reproducible binaries. The Makefile reads `.env` and injects
  values via `-ldflags` into `internal/buildcfg`

## Design decisions

- **CSV always flattens**: CSV output unconditionally flattens nested data (dot-notation for objects, row expansion for
  single array-of-objects, semicolons for primitive arrays). No `--no-flatten` for CSV — CSV is flat by nature, and
  `--output json` serves the nested-data use case. Table output respects `--flatten` flag since tables can show
  summaries like `"2 items"`
- **No per-error doc links**: API errors already include code, message, description, and request ID. Doc URLs rot, no
  stable CLI doc site exists, and no major CLI does this. A single help URL in `--help`/`version` suffices if needed
- **No structured logging**: CLI runs once and exits — structured logging is for long-running services. `--verbose` with
  stderr debug output is standard CLI pattern (`kubectl`, `gh`, `terraform`)
- **Generator is Python**: Python is better suited for YAML/JSON wrangling and code-gen string templating than Go.
  Dev-only tool, never shipped, not in critical path
- **Python linting with ruff**: Generator scripts use ruff with `select = ["ALL"]` and targeted ignores (no type
  annotations or docstrings for scripts-only context, complexity rules relaxed for inherently complex generator logic).
  Integrated into `make lint`, CI runs both Python tests and ruff
- **No edge case tests needed**: Unicode handled natively by Go + JSON, large responses bounded by API pagination,
  concurrent OAuth refreshes impossible (single-threaded CLI), config corruption already covered by
  `TestLoadMalformedJSON`
- **No binary-level E2E tests**: Generated command tests exercise the full path (flag parsing → HTTP → response
  formatting) with test servers. True binary E2E adds build/exec/parse overhead for marginal gain over existing coverage
