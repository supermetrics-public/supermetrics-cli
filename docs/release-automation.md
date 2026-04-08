# Release Automation

## GoReleaser Configuration (`.goreleaser.yaml`)

Key settings:

- **Builds**: Single binary named `supermetrics`, targeting 5 platform/arch combos:
  - `linux/amd64`, `linux/arm64`
  - `darwin/amd64`, `darwin/arm64`
  - `windows/amd64`
- **Ldflags**: Inject version, commit, and build date into `cmd.Version`, `cmd.Commit`, `cmd.BuildDate` (same ldflags
  pattern as the Makefile).
- **Archives**: `.tar.gz` for linux/darwin, `.zip` for windows. Each archive contains the binary plus `README.md` and
  `LICENSE`.
- **Checksum**: `checksums.txt` with SHA256 — used by `go-selfupdate` for verification during
  `supermetrics version upgrade`.
- **Changelog**: Auto-generated from conventional commit messages, grouped by type.
- **Homebrew tap**: Auto-publish formula to `supermetrics-public/homebrew-tap` repo. GoReleaser's `brews` section
  handles this natively — it pushes a formula file after each release. Requires a `HOMEBREW_TAP_TOKEN` secret (GitHub
  PAT with repo scope on the tap repo).
- **Docker** (optional, lower priority): Multi-arch image `supermetrics/cli:latest` pushed to GitHub Container Registry.
  Base image `gcr.io/distroless/static-debian12`. Useful for CI/CD pipelines that prefer containers.
- **Snapshot**: `name_template: "{{ .Tag }}-next"` for pre-release builds.

Before running: `make generate` must have been run (GoReleaser builds from existing source, doesn't run code generation
itself). The CI workflow handles this sequencing.

## Release Workflow (`.github/workflows/release.yml`)

**Trigger**: Push of a tag matching `v*` (e.g., `v0.1.0`).

```
Steps:
  1. Checkout with full history (fetch-depth: 0, needed for changelog)
  2. Setup Go 1.26
  3. Install tools: goimports
  4. make generate
  5. make test
  6. Run GoReleaser (goreleaser/goreleaser-action@v6)
     - Uses GITHUB_TOKEN for GitHub Releases
     - Uses HOMEBREW_TAP_TOKEN for Homebrew tap push
```

**Secrets needed**:
- `GITHUB_TOKEN` — automatic, used for creating the release
- `HOMEBREW_TAP_TOKEN` — GitHub PAT with `repo` scope on `supermetrics-public/homebrew-tap`, stored as a repo secret

## Homebrew Tap Repository (`supermetrics-public/homebrew-tap`)

Create a separate public repo `supermetrics-public/homebrew-tap`. GoReleaser auto-pushes the formula file on each
release. The formula:
- Downloads the correct archive for the user's OS/arch
- Verifies SHA256 checksum
- Installs the `supermetrics` binary
- Provides shell completion installation instructions in caveats

Users install with:
```bash
brew install supermetrics-public/tap/supermetrics
```

The tap repo needs no manual maintenance — GoReleaser updates it on every release.

## Spec Sync Workflow (`.github/workflows/spec-sync.yml`)

**Trigger**: `repository_dispatch` event with type `openapi-spec-updated`, sent by the SDK repo's CI when
`openapi-spec.yaml` changes on main.

```
Steps:
  1. Checkout supermetrics-cli main branch
  2. Fetch latest openapi-spec.yaml from SDK repo
     - Use: gh api repos/supermetrics-public/supermetrics-python-sdk/contents/openapi-spec.yaml
       --jq '.content' | base64 -d > openapi-spec.yaml
     - Or: curl the raw file URL from main branch
  3. Install tools: goimports
  4. make generate
  5. Check for diff in openapi-spec.yaml, cmd/generated/
  6. If no diff: exit (no changes needed)
  7. If diff: create a branch (e.g., auto/spec-update-<date>), commit, push
  8. Open PR with title "chore: sync OpenAPI spec from SDK" and body listing changed endpoints
  9. Optionally auto-merge if CI passes (configurable via branch protection)
```

**SDK repo side**: Add a step to the SDK's release/merge workflow that sends the dispatch:
```yaml
- name: Notify CLI repo
  uses: peter-evans/repository-dispatch@v3
  with:
    token: ${{ secrets.CLI_DISPATCH_TOKEN }}
    repository: supermetrics-public/supermetrics-cli
    event-type: openapi-spec-updated
```

Requires a `CLI_DISPATCH_TOKEN` secret in the SDK repo (GitHub PAT with `repo` scope on the CLI repo).

## Auto-Release Workflow (`.github/workflows/auto-release.yml`)

**Trigger**: Push to `main` that touches any of:
- `openapi-spec.yaml`
- `cmd/generated/**`
- `scripts/command-mapping.yaml`

```
Steps:
  1. Checkout with full history
  2. Determine current latest tag (e.g., v0.2.0)
  3. Determine bump type:
     - If new files in cmd/generated/ (new resource group): minor bump
     - Otherwise: patch bump
  4. Compute next version (e.g., v0.2.1 or v0.3.0)
  5. Create and push tag
  6. Tag push triggers release.yml → GoReleaser → binaries + Homebrew
```

This can use a lightweight action like `mathieudutour/github-tag-action` or a simple shell script with `git tag`.

**Safety**: The workflow only runs on main (not PRs), and only when generated code actually changed. A `[skip-release]`
marker in the commit message can bypass it.

## End-to-End Release Flow

```
1. OpenAPI spec updated in SDK repo (merged to main)
2. SDK CI sends repository_dispatch to supermetrics-cli
3. spec-sync.yml fetches new spec, runs make generate, opens PR
4. PR reviewed/merged (or auto-merged)
5. auto-release.yml detects generated code changes on main
6. New tag created (e.g., v0.3.1)
7. release.yml triggered by tag push
8. GoReleaser builds binaries for 5 platforms
9. GitHub Release published with binaries + checksums
10. Homebrew tap formula auto-updated
11. Users see "new version available" within a week (periodic check)
12. Users run "supermetrics version upgrade" → binary replaced in-place
```

## Implementation Checklist

- [x] Create `.goreleaser.yaml` with builds, archives, checksum, brews sections
- [x] Create `.github/workflows/release.yml` (tag-triggered, runs GoReleaser)
- [x] Create `.github/workflows/spec-sync.yml` (repository_dispatch handler)
- [x] Create `.github/workflows/auto-release.yml` (tag bumper on generated code changes)
- [ ] Create `supermetrics-public/homebrew-tap` repo (empty, GoReleaser populates it) — **requires GitHub admin**
- [ ] Add dispatch step to SDK repo's CI workflow — **requires SDK repo access**
- [ ] Set up repo secrets: `HOMEBREW_TAP_TOKEN`, `CLI_DISPATCH_TOKEN` — **requires GitHub admin**
- [ ] Test: push a tag manually, verify GoReleaser produces correct artifacts
- [ ] Test: trigger spec-sync manually via `gh api`, verify PR is created
- [ ] Test: merge spec-sync PR, verify auto-release creates tag and release

## API Change Handling

### How each type of API change is handled

| Change                                      | Automation level                 | Version bump  | Manual steps                                                  |
|---------------------------------------------|----------------------------------|---------------|---------------------------------------------------------------|
| New optional parameter on existing endpoint | Fully automatic                  | Patch         | None                                                          |
| New endpoint on existing resource           | Semi-automatic                   | Patch         | Add ~3 lines to `command-mapping.yaml`                        |
| New resource group                          | Semi-automatic                   | **Minor**     | Add mapping entry with resource name, server_index, commands  |
| Parameter renamed/removed (breaking)        | Automatic but silent             | Patch         | Review spec-sync PR diff for user impact                      |
| Endpoint removed                            | Automatic (skipped with warning) | Patch         | Clean up orphaned mapping entry                               |
| Server-side bug fix (no spec change)        | No CLI release needed            | —             | None                                                          |
| CLI bug fix (non-generated code)            | Not auto-released                | Patch         | Push tag manually: `git tag v0.X.Y && git push origin v0.X.Y` |

### Version bumping rules

- **Minor** (v0.3.x → v0.4.0): New files added in `cmd/generated/` (= new resource group)
- **Patch** (v0.3.0 → v0.3.1): All other generated code changes
- **Manual tag**: Changes to hand-written code (`internal/`, `cmd/root.go`, etc.) — intentionally not auto-released to
  avoid accidental releases from refactoring
- **Skip**: Commit message containing `[skip-release]` bypasses auto-release

### Known gaps

1. **No breaking change detection.** If the API renames a parameter, the CLI silently swaps the flag name. Users'
   scripts break with no warning. A future improvement: diff generated commands in the spec-sync PR and flag
   removed/renamed flags.

2. **No meaningful changelog for spec syncs.** All spec-sync PRs have the same commit message ("chore: sync OpenAPI spec
   from SDK"). The PR body could be enhanced to list which operations changed.

3. **Infrastructure not yet provisioned.** The Homebrew tap repo (`supermetrics-public/homebrew-tap`), repo secrets
   (`HOMEBREW_TAP_TOKEN`, `CLI_DISPATCH_TOKEN`), and the SDK-side dispatch step all require GitHub admin setup. See the
   implementation checklist above.
