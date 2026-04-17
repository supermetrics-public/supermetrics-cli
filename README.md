# Supermetrics CLI

> **Alpha Release** — This CLI is under active development. Features, flags, and behavior may change without notice.
> If you encounter bugs or need help, please [open a GitHub issue](https://github.com/supermetrics-public/supermetrics-cli/issues).

Command-line interface for the [Supermetrics API](https://supermetrics.com). Query marketing data, manage OAuth login
links, schedule Data Warehouse backfills, and more from your terminal or scripts.

## Installation

### Homebrew (macOS / Linux)

> **Note:** The Homebrew tap is not yet available. Use one of the other installation methods below.

```bash
brew install supermetrics-public/tap/supermetrics
```

### Download from GitHub Releases

Download the latest binary for your platform from the
[Releases](https://github.com/supermetrics-public/supermetrics-cli/releases) page.

### Using `go install`

```bash
go install github.com/supermetrics-public/supermetrics-cli/cmd/supermetrics@latest
```

### From source

```bash
git clone https://github.com/supermetrics-public/supermetrics-cli.git
cd supermetrics-cli
make build
# Binary is at ./bin/supermetrics
```

## Quick start

1. Log in with your Supermetrics account:

   ```bash
   supermetrics login
   ```

   Or configure an API key instead:

   ```bash
   supermetrics configure
   ```

2. Run a query:

   ```bash
   supermetrics queries execute \
     --ds-id ga4 \
     --fields sessions \
     --start-date 2024-01-01 \
     --end-date 2024-01-31
   ```

   For large result sets, use `--all` to auto-paginate or `--limit N` to cap rows.
   Combine with `--max-rows` to also limit server-side for faster queries:

   ```bash
   supermetrics queries execute --ds-id ga4 --fields sessions --start-date 2024-01-01 --end-date 2024-01-31 --all
   supermetrics queries execute --ds-id ga4 --fields sessions --start-date 2024-01-01 --end-date 2024-01-31 --limit 500 --max-rows 500
   ```

3. List accounts for a data source:

   ```bash
   supermetrics accounts list --ds-id ga4
   ```

## Authentication

The CLI supports two authentication methods: **OAuth login** (recommended) and **API keys**.

### OAuth login (recommended)

```bash
supermetrics login           # Opens browser for Google/Microsoft login
supermetrics login --status  # Show current auth status
supermetrics logout          # Revoke tokens and log out
```

OAuth tokens are stored in the config file and refreshed automatically when they expire.

### API keys

```bash
supermetrics configure       # Set up API key interactively
# or
export SUPERMETRICS_API_KEY=your-key
# or
supermetrics accounts list --api-key your-key
```

### Resolution priority

The CLI resolves credentials in this order (highest first):

1. `--api-key` flag
2. `SUPERMETRICS_API_KEY` environment variable
3. OAuth token from config (auto-refreshed if expired)
4. API key from config file

If no credentials are found, the CLI suggests running `supermetrics login` or `supermetrics configure`.

## Commands

| Command                        | Description                          |
|--------------------------------|--------------------------------------|
| `supermetrics login`           | Log in via OAuth (Google/Microsoft)  |
| `supermetrics logout`          | Revoke tokens and log out            |
| `supermetrics configure`       | Set up API key and defaults          |
| `supermetrics version`         | Show version, build date, and commit |
| `supermetrics version upgrade` | Self-update to the latest release    |
| `supermetrics completion`      | Generate shell completion scripts    |
| `supermetrics man <dir>`       | Generate man pages                   |

### Resource commands

| Resource      | Subcommands                                                | Description                                             |
|---------------|------------------------------------------------------------|---------------------------------------------------------|
| `login-links` | `create`, `get`, `list`, `close`                           | Manage OAuth login links for data source authentication |
| `logins`      | `get`, `list`                                              | View authenticated data source logins                   |
| `accounts`    | `list`                                                     | List available accounts for a data source               |
| `queries`     | `execute`                                                  | Execute data queries against connected sources          |
| `backfills`   | `create`, `get`, `get-latest`, `list-incomplete`, `cancel` | Manage Data Warehouse backfills                         |
| `datasource`  | `get`                                                      | View data source configuration details                  |

Use `--help` on any command for detailed flag information:

```bash
supermetrics queries execute --help
supermetrics backfills --help
```

## Output formats

All commands support three output formats via `--output` / `-o`:

| Format           | Description                                                           |
|------------------|-----------------------------------------------------------------------|
| `json` (default) | Pretty-printed JSON with syntax coloring, suitable for piping to `jq` |
| `table`          | Human-readable table with box-drawing borders and colored headers     |
| `csv`            | Standard CSV with nested data automatically flattened                 |

The CLI automatically unwraps the API response envelope and outputs the `data` payload directly. Use `--verbose` to see
the full request/response details including the API `request_id` (useful for troubleshooting with Supermetrics support).

### Nested data handling

API responses often contain nested objects and arrays. Each format handles them differently:

- **JSON**: Full nested structure preserved as-is.
- **Table**: Nested arrays show as summaries (e.g., `53 items`). Use `--flatten` to expand nested data into individual
  rows.
- **CSV**: Nested data is always flattened automatically:
  - Arrays of objects are expanded into multiple rows (parent fields repeated)
  - Nested objects use dot-notation columns (e.g., `ds_info.ds_id`)
  - Primitive arrays are joined with semicolons (e.g., `read;write;admin`)

```bash
supermetrics logins list -o table
supermetrics accounts list --ds-id ga4 -o csv > accounts.csv
supermetrics accounts list --ds-id ga4 -o table --flatten
```

### Field selection

Use `--fields` to include only specific fields in the output. Dot-notation is supported for nested fields:

```bash
supermetrics logins list --fields login_id,ds_info.name,display_name
supermetrics datasource get --data-source-id GAWA --fields name,status,account_labels.singular
```

Works with all output formats (JSON, table, CSV). Note: `queries execute` has its own `--fields` flag that selects
fields server-side — the global `--fields` does not apply to that command.

### Named profiles

Store multiple credential sets and switch between them:

```bash
# Set up credentials for different contexts
supermetrics configure                     # default profile
supermetrics configure --profile work      # work profile
supermetrics login --profile staging       # staging profile with OAuth

# List profiles (* = active)
supermetrics profile list

# Switch the active profile
supermetrics profile use work

# Per-command override without switching
supermetrics logins list --profile staging

# View profile details
supermetrics profile show work

# Delete a profile
supermetrics profile delete staging
```

Profiles hold credentials only (API key + OAuth tokens). Settings like `--output` are global. The active profile can
also be set via the `SUPERMETRICS_PROFILE` environment variable.

## Command features

### Dry-run mode

The `--dry-run` flag prints the request details (method, URL, body) to stderr without executing.

Available on: `login-links create`, `backfills create`, `backfills cancel`

```bash
supermetrics backfills create --team-id 1 --transfer-id 2 --range-start 2025-01-01 --range-end 2025-01-31 --dry-run
```

### Confirmation prompts

Destructive commands prompt for confirmation before executing. Use `-y` / `--yes` to skip the prompt (also skipped
automatically when stdin is not a terminal).

Available on: `login-links close`, `backfills cancel`

```bash
supermetrics backfills cancel --backfill-id 123          # Prompts: "Cancel backfill 123? [y/N]:"
supermetrics backfills cancel --backfill-id 123 --yes    # Skips prompt
```

### Wait for completion

`backfills create --wait` polls until the backfill completes, showing a progress bar with transfer run counts. Times out
after 60 minutes.

```bash
supermetrics backfills create --team-id 1 --transfer-id 2 --range-start 2025-01-01 --range-end 2025-01-31 --wait
```

### Progress indicators

All commands show an animated spinner on stderr with elapsed time while waiting for a response. Additionally:

- `queries execute` polls for results automatically with adaptive intervals (1s → 2s → 5s). With `--all` or `--limit`,
  fetches multiple pages and reports progress to stderr
- `backfills create --wait` shows a determinate progress bar with transfer run counts

Progress output is suppressed in `--quiet` mode, `--verbose` mode, or when stderr is not a terminal.

## Configuration

Running `supermetrics configure` creates a config file at `~/.config/supermetrics/config.json` with the following
settings:

| Setting                      | Description                | Default  |
|------------------------------|----------------------------|----------|
| `api_key`                    | API key for authentication | _(none)_ |
| `default_output`             | Default output format      | `json`   |
| `update_check_interval_days` | Days between update checks | `7`      |

The config file is created with `0600` permissions (user-only read/write).

## Environment variables

| Variable                       | Description                                                    |
|--------------------------------|----------------------------------------------------------------|
| `SUPERMETRICS_API_KEY`         | API key (overridden by `--api-key` flag)                       |
| `SUPERMETRICS_DOMAIN`          | API domain override (default: `supermetrics.com`)              |
| `SUPERMETRICS_OAUTH_CLIENT_ID` | OAuth client ID (baked in at build time for releases)          |
| `SUPERMETRICS_OAUTH_SCOPES`    | OAuth scopes (baked in at build time for releases)             |
| `SUPERMETRICS_NO_UPDATE_CHECK` | Set to `1` to disable background update checks                 |
| `SUPERMETRICS_PROFILE`         | Named profile to use (overridden by `--profile` flag)          |
| `SUPERMETRICS_QUIET`           | Set to non-empty, non-`0` value to enable quiet mode           |
| `NO_COLOR`                     | Disable colored output ([no-color.org](https://no-color.org/)) |

## Global flags

| Flag         | Short  | Description                                                                  |
|--------------|--------|------------------------------------------------------------------------------|
| `--api-key`  |        | Override API key                                                             |
| `--output`   | `-o`   | Output format: `json`, `table`, `csv`                                        |
| `--verbose`  | `-v`   | Enable verbose output (request/response details, request IDs)                |
| `--fields`   |        | Comma-separated list of fields to include in output (dot-notation supported) |
| `--flatten`  |        | Expand nested data in table output (CSV always flattens)                     |
| `--no-color` |        | Disable colored output                                                       |
| `--no-retry` |        | Disable automatic retry on transient errors                                  |
| `--profile`  |        | Named profile to use for credentials                                         |
| `--quiet`    | `-q`   | Suppress informational output (spinners, update hints)                       |
| `--timeout`  |        | Override request timeout (e.g., `30s`, `5m`, `1h`)                           |

## Retry behavior

The CLI automatically retries transient failures (HTTP 429, 500, 502, 503, 504, and network errors):

- **3 total attempts** with exponential backoff and full jitter
- Base delay 500ms, maximum 20s
- Honors `Retry-After` header on 429 rate-limit responses
- Disable with `--no-retry`
- Use `--verbose` to see retry logs on stderr

## Exit codes

| Code | Meaning     | Description                                        |
|------|-------------|----------------------------------------------------|
| 0    | Success     | Command completed successfully                     |
| 1    | General     | Unclassified error (API error, config error, etc.) |
| 64   | Usage       | Invalid flags or missing required arguments        |
| 65   | Auth        | Authentication or authorization failure            |
| 69   | Unavailable | Network error, timeout, or service unavailable     |

```bash
supermetrics queries execute ... || case $? in
  65) echo "Auth failed"; supermetrics login ;;
  69) echo "Service unavailable" ;;
esac
```

## Shell completion

Generate completion scripts for your shell:

```bash
# Bash
source <(supermetrics completion bash)

# Zsh
supermetrics completion zsh > "${fpath[1]}/_supermetrics"

# Fish
supermetrics completion fish | source

# PowerShell
supermetrics completion powershell | Out-String | Invoke-Expression
```

Run `supermetrics completion --help` for per-shell instructions on persisting completions across sessions.

## Proxy support

The CLI respects the standard HTTP proxy environment variables:

- `HTTP_PROXY` / `http_proxy`
- `HTTPS_PROXY` / `https_proxy`
- `NO_PROXY` / `no_proxy`

These are handled by Go's default HTTP transport, so no additional configuration is needed.

## Updating

The CLI checks for new versions in the background (every 7 days by default, only in interactive terminals). When an
update is available, a one-line hint is printed to stderr.

```bash
# Check for updates
supermetrics version upgrade --check

# Upgrade to the latest version
supermetrics version upgrade

# Force reinstall current version
supermetrics version upgrade --force
```

If installed via Homebrew, the CLI will direct you to use `brew upgrade` instead.

## Support

This CLI is in **Alpha**. If you run into issues, have questions, or want to request a feature, please open an issue:

https://github.com/supermetrics-public/supermetrics-cli/issues

---

## Development

### Prerequisites

- Go 1.26+
- Python 3.13+ (for command generation, pinned in `.python-version`)
- [uv](https://docs.astral.sh/uv/) (Python package manager, used for running generator and linter)

Install all required tools (Go tools, golangci-lint, ruff) with:

```bash
make tools
```

### Environment setup

Copy `.env.example` to `.env` for local development. The Makefile reads `.env` and injects the values into the binary
via `-ldflags` at build time. Release builds get these values from CI secrets instead.

```bash
cp .env.example .env
```

### Make targets

```bash
make build          # Build binary with version info (output: bin/supermetrics)
make build-release  # Build with stripped symbols and debug info
make run            # Run without building (supports ARGS="queries --help")
make install        # Install to $GOPATH/bin
make test           # Run all tests (Go + Python generator)
make test-go        # Run Go tests only
make test-python    # Run Python generator tests only
make test-coverage  # Run tests with race detection and coverage report
make vet            # Run go vet static analysis
make lint           # Run golangci-lint + ruff (Python scripts)
make lint-python    # Run ruff only (Python scripts)
make lint-fix       # Auto-fix lint and formatting issues (Go + Python)
make vulncheck      # Scan for known vulnerabilities
make tidy-check     # Verify go.mod and go.sum are tidy
make generate       # Regenerate code from OpenAPI spec
make tools          # Install all required tools (Go tools, golangci-lint, ruff)
make snapshot       # Test GoReleaser build locally (no publish)
make clean          # Remove bin/ and coverage.out
```

### Code generation

CLI commands are generated from the OpenAPI spec:

- **Cobra commands** — `scripts/generate_commands.py` reads `openapi-spec.yaml` + `scripts/command-mapping.yaml` and
  generates one file per resource group into `cmd/generated/`.

The mapping file (`scripts/command-mapping.yaml`) controls which OpenAPI operations become CLI commands and how they are
grouped.

To add or modify commands, edit `command-mapping.yaml` and run `make generate`.

The `openapi-spec.yaml` is synced from the Python SDK repo via CI. When the SDK updates the spec, a
`repository_dispatch` event triggers the spec-sync workflow, which regenerates code and opens a PR. See
[docs/release-automation.md](docs/release-automation.md) for the full release pipeline.

### Project structure

```
supermetrics-cli/
├── cmd/
│   ├── supermetrics/              Entry point (go install target)
│   │   └── main.go
│   ├── root.go                    Root command, global flags, config loading
│   ├── version.go                 Version and upgrade commands
│   ├── configure.go               Interactive config setup
│   ├── completion.go              Shell completion generation
│   ├── man.go                     Man page generation
│   └── generated/                 Auto-generated resource commands (do not edit)
│       └── register.go            Command registration and shared helpers
├── internal/
│   ├── auth/                      Auth resolution (OAuth PKCE + API key) and token refresh
│   ├── buildcfg/                  Build-time injected values (version, OAuth client config)
│   ├── config/                    Config file management and validation
│   ├── httpclient/                Shared HTTP client (timeouts, auth, envelope, logging)
│   ├── output/                    JSON/table/CSV output formatting with color
│   └── update/                    Update checking and self-update
├── scripts/
│   ├── generate_commands.py       Command generator (runs via uv run python3)
│   ├── test_generate_commands.py  Generator unit tests (48 tests)
│   └── command-mapping.yaml       OpenAPI-to-CLI mapping
├── openapi-spec.yaml              Supermetrics API specification
├── pyproject.toml                 Python project config (ruff linting rules)
├── .python-version                Pinned Python version (3.13)
├── .env.example                   Development environment variables (copy to .env)
├── .goreleaser.yaml               Cross-platform release configuration
├── .golangci.yml                  Go linting configuration
├── .golangci-lint-version         Pinned golangci-lint version
├── .covignore                     Packages excluded from coverage reports
└── Makefile                       Build, test, lint, generate targets
```

### Testing

```bash
make test           # All tests (Go + Python), no race detector
make test-go        # Go tests only
make test-python    # Python generator tests only
make test-coverage  # Go tests with race detection + coverage summary
```

Tests cover authentication resolution, config persistence and file permissions, config validation, HTTP client behavior
(timeouts, errors, auth headers, response envelope handling, retry logic), output formatting, generated command
integration (flag parsing, URL building, query encoding, output formats), exit code classification, update checking
logic, and the Python code generator (48 unit tests for helpers, parameter extraction, and output structure).
