# AGENTS.md

Supermetrics CLI (`supermetrics`) — a Go CLI for the Supermetrics API. Single binary, no runtime dependencies.

## Commands

```bash
cp .env.example .env                      # One-time: dev environment variables
make tools                                # One-time: install all dev tools
make build                                # Build to bin/supermetrics
make run ARGS="queries execute --help"    # Run without building
make test                                 # All tests (Go + Python generator)
make test-go                              # Go tests only
make test-python                          # Python generator tests only (via uv)
make test-coverage                        # Race detector + coverage report
make lint                                 # golangci-lint + ruff
make lint-fix                             # Auto-fix lint and formatting
make generate                             # Regenerate CLI commands from OpenAPI spec
make vet                                  # Static analysis
make vulncheck                            # Known vulnerability scan
```

## Boundaries

**Never do:**
- Edit files in `cmd/generated/` — they are overwritten by `make generate`
- Use `http.DefaultClient` — all HTTP goes through `internal/httpclient.Do()`
- Commit secrets or `.env` files

**Ask first:**
- Changes to `scripts/command-mapping.yaml` (drives code generation)
- Changes to `.goreleaser.yaml` or CI workflows
- Adding new dependencies

## Code generation

Commands are generated from `openapi-spec.yaml` + `scripts/command-mapping.yaml`.
To add a CLI command: add an entry to `scripts/command-mapping.yaml` and run `make generate`.

The generator (`scripts/generate_commands.py`) runs via `uv run python3` (Python 3.13+, pinned in `.python-version`).
It emits all potentially needed imports — `goimports` cleans up unused ones.

Per-command features in `command-mapping.yaml`:
- `dry_run`, `confirm`, `fixed_values`, `exclude_params`, `async`, `wait`, `spinner_text`, `timeout`, `paginated`

## Non-obvious conventions

**API envelope unwrapping:** The Supermetrics API wraps all responses in `{meta: {request_id}, data: ...}`.
`Response.ParseJSON()` unwraps automatically — callers get just the `data` payload. API errors (including HTTP 200
with `error` field) return Go errors with code, message, and request ID.

**Two `--fields` flags coexist:** The global `--fields` (string, comma-separated) does client-side field filtering.
`queries execute` has its own local `--fields` (StringSlice) for server-side selection. Cobra flag shadowing ensures
the local flag wins — when `queries execute --fields` is used, the global persistent `--fields` stays empty.

**CSV always flattens:** No `--no-flatten` for CSV. Dot-notation for nested objects, row expansion for arrays of
objects, semicolons for primitive arrays. Use `--output json` for nested data.

**Auth resolution order:** `--api-key` flag > `SUPERMETRICS_API_KEY` env > OAuth token (auto-refreshed) > API key
from config. OAuth uses Authorization Code + PKCE. Client ID and scopes are injected at build time via
`internal/buildcfg` (from `.env` locally, from CI secrets in releases).

**Profile resolution:** `--profile` flag > `SUPERMETRICS_PROFILE` env > `active_profile` in config > `"default"`.

**Build-time injection:** `CGO_ENABLED=0`, `-trimpath`. The Makefile reads `.env` and injects values via `-ldflags`
into `internal/buildcfg`. Release builds get these from CI secrets via GoReleaser.

**Timeout defaults:** 30s default. Long-running commands (queries, accounts) use 60m — set via `timeout` field in
`command-mapping.yaml`. Users override with `--timeout` (Go duration format: `30s`, `5m`, `1h`).

## Code style

```go
// Do: return errors with context from httpclient
resp, err := httpclient.Do(ctx, req, httpclient.Options{Timeout: timeout})
if err != nil {
    return fmt.Errorf("failed to list accounts: %w", err)
}

// Do: unwrap API envelope
var accounts []Account
if err := resp.ParseJSON(&accounts); err != nil {
    return err
}

// Don't: use http.DefaultClient directly
// Don't: manually parse the API envelope {meta, data, error}
```

## CI and releases

Releases trigger on `v*` tag push. GoReleaser builds for linux/darwin (amd64+arm64) and windows (amd64).

Required CI secrets: `HOMEBREW_TAP_TOKEN` (PAT for `supermetrics-public/homebrew-tap`),
`SUPERMETRICS_OAUTH_CLIENT_ID`, `SUPERMETRICS_OAUTH_SCOPES`.

OpenAPI spec syncs from the SDK repo via `repository_dispatch` -> spec-sync workflow -> PR.
Auto-release workflow tags patch/minor bumps when generated code changes on main.
