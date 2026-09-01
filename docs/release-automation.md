# Release Automation

## GoReleaser Configuration (`.goreleaser.yaml`)

Key settings:

- **Builds**: Single binary named `supermetrics`, targeting 5 platform/arch combos:
  - `linux/amd64`, `linux/arm64`
  - `darwin/amd64`, `darwin/arm64`
  - `windows/amd64`
- **Ldflags**: Inject version, commit, build date and the OAuth client config into `internal/buildcfg` (same ldflags
  pattern as the Makefile).
- **Archives**: `.tar.gz` for linux/darwin, `.zip` for windows. Each archive contains the binary plus `README.md` and
  `LICENSE`.
- **Checksum**: `checksums.txt` with SHA256 — `internal/update` verifies the downloaded archive against it during
  `supermetrics version upgrade`, then hands the binary to `minio/selfupdate` for the atomic replace.
- **Changelog**: Auto-generated from conventional commit messages, grouped by type.
- **Homebrew tap**: Auto-publish formula to `supermetrics-public/homebrew-tap` repo. GoReleaser's `brews` section
  handles this natively — it pushes a formula file after each release. Requires a `HOMEBREW_TAP_TOKEN` secret (GitHub
  PAT with repo scope on the tap repo).
- **Docker** — *not configured; proposal only.* Would publish a multi-arch `supermetrics/cli:latest` to GitHub
  Container Registry on `gcr.io/distroless/static-debian12`, for CI/CD pipelines that prefer containers.
- **Snapshot**: `name_template: "{{ .Tag }}-next"` for pre-release builds.

Before running: `make generate` must have been run (GoReleaser builds from existing source, doesn't run code generation
itself). The CI workflow handles this sequencing.

## Release Workflow (`.github/workflows/release.yml`)

**Trigger**: Manual `workflow_dispatch` with a required `version` input (e.g. `v0.4.0`). The workflow creates and
pushes the tag itself — it is not triggered by a tag push.

```
Steps:
  1. Validate the version input matches vMAJOR.MINOR.PATCH
  2. Checkout with full history (fetch-depth: 0, needed for changelog)
  3. Create and push the tag
  4. Install toolchain via mise (Go, Python, uv, goimports)
  5. make generate
  6. make test
  7. Run GoReleaser (goreleaser/goreleaser-action, args: release --clean)
     - Uses GITHUB_TOKEN for GitHub Releases
     - Uses HOMEBREW_TAP_TOKEN for Homebrew tap push
     - Uses SUPERMETRICS_OAUTH_CLIENT_ID / SUPERMETRICS_OAUTH_SCOPES for the ldflags-injected build config
  8. Notify the linux-packages repo via repository_dispatch
```

**Secrets needed**:
- `GITHUB_TOKEN` — automatic, used for creating the release
- `HOMEBREW_TAP_TOKEN` — GitHub PAT with `repo` scope on `supermetrics-public/homebrew-tap`, stored as a repo secret
  and in 1Password (`GitHub PAT — Homebrew Tap`)
- `LINUX_PACKAGES_TOKEN` — GitHub PAT with `repo` scope on `supermetrics-public/linux-packages`, stored as a repo
  secret and in 1Password (`GitHub PAT — Linux Packages`)
- `SUPERMETRICS_OAUTH_CLIENT_ID` and `SUPERMETRICS_OAUTH_SCOPES` — injected into `internal/buildcfg` via `-ldflags`;
  locally these come from `.env` instead

## Homebrew Tap Repository (`supermetrics-public/homebrew-tap`)

GoReleaser auto-pushes the formula file on each release. The formula:
- Downloads the correct archive for the user's OS/arch
- Verifies SHA256 checksum
- Installs the `supermetrics` binary
- Provides shell completion installation instructions in caveats

Users install with:
```bash
brew install supermetrics-public/tap/supermetrics
```

The tap repo needs no manual maintenance — GoReleaser updates it on every release.

### Repo setup instructions

1. **Create the repo**: `supermetrics-public/homebrew-tap` — **public** (required for `brew install` to work without
   authentication; Homebrew taps must be public repos)
2. **Initialize**: The repo can be completely empty — GoReleaser creates and pushes the formula file automatically on
   the first release
3. **GitHub PAT**: Create a fine-grained PAT or classic PAT with `repo` scope on `homebrew-tap`:
   - Store in 1Password as `GitHub PAT — Homebrew Tap`
   - Add as `HOMEBREW_TAP_TOKEN` secret in `supermetrics-cli` repo
   - GoReleaser uses this token to push the formula file (see `brews` section in `.goreleaser.yaml`)

**Security and branch protection settings:**

| Setting                              | Value    | Why                                                                    |
|--------------------------------------|----------|------------------------------------------------------------------------|
| Visibility                           | Public   | Required for `brew install` to work                                    |
| Default branch                       | `main`   |                                                                        |
| Require pull request before merging  | Off      | GoReleaser pushes directly to the default branch                       |
| Require status checks                | Off      | No CI needed — the formula is generated, not hand-written              |
| Allow force pushes                   | Off      | Prevent accidental history rewrites                                    |
| Allow deletions                      | Off      |                                                                        |
| Restrict who can push                | Optional | Lock down to the PAT's user account + org admins if desired            |
| Require signed commits               | Off      | GoReleaser does not sign commits                                       |

**No additional workflows, secrets, or environments are needed in this repo.** GoReleaser in the CLI repo handles
everything via the PAT.

## Linux Packages (deb / rpm / apk)

GoReleaser's `nfpms` section builds `.deb`, `.rpm`, and `.apk` packages for every release. These are uploaded to the
GitHub Release alongside the tarballs and checksums.

### Direct install from GitHub Releases

Users can download and install packages directly (no repo setup needed):

```bash
# Debian / Ubuntu
curl -LO https://github.com/supermetrics-public/supermetrics-cli/releases/latest/download/supermetrics_<version>_linux_amd64.deb
sudo dpkg -i supermetrics_<version>_linux_amd64.deb

# RHEL / Fedora / Amazon Linux
curl -LO https://github.com/supermetrics-public/supermetrics-cli/releases/latest/download/supermetrics_<version>_linux_amd64.rpm
sudo rpm -i supermetrics_<version>_linux_amd64.rpm

# Alpine Linux
curl -LO https://github.com/supermetrics-public/supermetrics-cli/releases/latest/download/supermetrics_<version>_linux_amd64.apk
sudo apk add --allow-untrusted supermetrics_<version>_linux_amd64.apk
```

### Linux Package Repository (`supermetrics-public/linux-packages`)

APT and YUM/DNF repos are hosted on GitHub Pages via a separate repo, following the same pattern as the Homebrew tap
(`supermetrics-public/homebrew-tap`). GoReleaser uploads `.deb`/`.rpm` to the GitHub Release, then a
`repository_dispatch` triggers the `linux-packages` repo to rebuild the APT and YUM repo metadata and deploy to
GitHub Pages.

**Base URL**: `https://supermetrics-public.github.io/linux-packages/`

#### 1. GPG signing key

APT and YUM repos require signed metadata. Generate a dedicated key for CI:

```bash
# Generate key (no passphrase, for CI use)
gpg --batch --gen-key <<EOF
%no-protection
Key-Type: RSA
Key-Length: 4096
Name-Real: Supermetrics CLI
Name-Email: cli@supermetrics.com
Expire-Date: 0
EOF

# Find the 40-char fingerprint
gpg --list-keys "cli@supermetrics.com"
# Example output: ABCD1234EFGH5678...

# Export private key → GitHub secret + 1Password
gpg --armor --export-secret-keys cli@supermetrics.com > private.asc

# Export binary public key → committed to linux-packages repo
gpg --export cli@supermetrics.com > pubkey.gpg
```

**1Password storage:** Store both keys and the fingerprint in 1Password before proceeding:

1. Create a **Secure Note** in 1Password (e.g., in a shared "Supermetrics CLI / CI" vault):
   - **Title**: `GPG Signing Key — Linux Packages`
   - **Fields**:
     - `Private Key (armored)` — paste contents of `private.asc`
     - `Key Fingerprint` — the 40-char fingerprint
   - **Attachments**: attach `pubkey.gpg` (binary public key)
2. Copy the private key and fingerprint from 1Password into the GitHub secrets/variables (see step 2 below)
3. **Delete `private.asc` from your local machine** — 1Password is now the source of truth for recovery

#### 2. Create the `linux-packages` repo

Create a new **public** repo: `supermetrics-public/linux-packages` (same pattern as `homebrew-tap` — an infrastructure
repo populated automatically by CI).

**Why public?** GitHub Pages for free requires a public repo. The repo only contains package metadata, the public GPG
key, and CI workflows — no proprietary code or credentials.

##### Repository structure

```
linux-packages/
├── .github/workflows/
│   └── update-repo.yml    ← rebuilds APT + YUM metadata, deploys to Pages
├── pubkey.gpg              ← binary public GPG key (committed)
└── index.html              ← optional landing page for browser visitors
```

##### Configuration steps

1. **GitHub Pages**: Settings → Pages → Source → **GitHub Actions**
2. **Secrets** (Settings → Secrets and variables → Actions):

   | Name                 | Type     | Value                                       | 1Password source                     |
   |----------------------|----------|---------------------------------------------|--------------------------------------|
   | `GPG_PRIVATE_KEY`    | Secret   | Contents of `private.asc` (armored key)     | `GPG Signing Key — Linux Packages`   |
   | `KEY_ID`             | Variable | 40-char GPG fingerprint                     | `GPG Signing Key — Linux Packages`   |

3. **Environment**: Create an environment named `github-pages` (required by `deploy-pages` action).
   No additional environment protection rules needed — the workflow only runs on `repository_dispatch`
   (triggered by the CLI repo's release workflow) or manual `workflow_dispatch`

##### Security and branch protection settings

| Setting                              | Value    | Why                                                                    |
|--------------------------------------|----------|------------------------------------------------------------------------|
| Visibility                           | Public   | Required for free GitHub Pages hosting                                 |
| Default branch                       | `main`   |                                                                        |
| Require pull request before merging  | On       | Workflow and config changes should be reviewed                         |
| Require approvals                    | 1        | At least one reviewer for workflow changes                             |
| Require status checks                | Off      | No CI tests in this repo                                               |
| Allow force pushes                   | Off      | Prevent accidental history rewrites                                    |
| Allow deletions                      | Off      |                                                                        |
| Restrict who can push                | Optional | Lock to org admins if desired                                          |
| Require signed commits               | Off      |                                                                        |

**Note on the `github-pages` environment:** GitHub automatically creates deployment protection for GitHub Pages.
The `deploy-pages` action requires the `id-token: write` permission and the `github-pages` environment. No additional
approval gates are needed — the deployment is only triggered by `repository_dispatch` from the CLI repo (requires
the `LINUX_PACKAGES_TOKEN` PAT) or by a manual `workflow_dispatch` (requires write access to the repo).

Unlike `homebrew-tap`, this repo has actual committed files (`pubkey.gpg`, workflow, optional `index.html`), so
branch protection with PR reviews is recommended to prevent accidental changes to the signing key or workflow.

#### 3. Update workflow in `linux-packages` repo

Create `.github/workflows/update-repo.yml`:

```yaml
name: Update package repos

on:
  # Triggered by supermetrics-cli release.yml after GoReleaser finishes
  repository_dispatch:
    types: [release-published]
  # Manual trigger for testing or rebuilding
  workflow_dispatch:
    inputs:
      tag:
        description: "Release tag (e.g. v0.3.1)"
        required: true

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  update-repos:
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    env:
      GPG_PRIVATE_KEY: ${{ secrets.GPG_PRIVATE_KEY }}
      KEY_ID: ${{ vars.KEY_ID }}
    steps:
      - uses: actions/checkout@v4

      - name: Determine version
        id: version
        run: |
          TAG="${{ github.event.client_payload.tag || inputs.tag }}"
          VERSION="${TAG#v}"
          echo "tag=$TAG" >> "$GITHUB_OUTPUT"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"

      - name: Install tools
        run: sudo apt-get update && sudo apt-get install -y reprepro createrepo-c

      - name: Import GPG key
        run: |
          echo "$GPG_PRIVATE_KEY" | gpg --batch --import
          echo "$KEY_ID:6:" | gpg --batch --import-ownertrust

      - name: Download .deb and .rpm packages from release
        run: |
          mkdir -p downloads
          gh release download "${{ steps.version.outputs.tag }}" \
            --repo supermetrics-public/supermetrics-cli \
            --pattern "*.deb" \
            --pattern "*.rpm" \
            --dir downloads
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      # ---------- APT repo (served at root: /dists/, /pool/) ----------
      - name: Build APT repo
        run: |
          mkdir -p apt-repo/conf

          cat > apt-repo/conf/distributions <<DISTCONF
          Origin: Supermetrics
          Label: Supermetrics CLI
          Suite: stable
          Codename: stable
          Components: main
          Architectures: amd64 arm64
          SignWith: $KEY_ID
          DISTCONF

          for deb in downloads/*.deb; do
            reprepro -Vb apt-repo includedeb stable "$deb"
          done

          cp pubkey.gpg apt-repo/pubkey.gpg

      # ---------- YUM/DNF repo (served at /yum/) ----------
      - name: Build YUM repo
        run: |
          mkdir -p yum-repo/packages
          cp downloads/*.rpm yum-repo/packages/
          createrepo_c yum-repo
          gpg --batch --yes --detach-sign --armor yum-repo/repodata/repomd.xml

      # ---------- Combine and deploy to GitHub Pages ----------
      - name: Assemble site
        run: |
          mkdir -p site/yum
          cp -r apt-repo/* site/
          cp -r yum-repo/* site/yum/
          cp pubkey.gpg site/
          [ -f index.html ] && cp index.html site/

      - uses: actions/configure-pages@v4
      - uses: actions/upload-pages-artifact@v3
        with:
          path: site
      - uses: actions/deploy-pages@v4
        id: deployment
```

#### 4. Trigger from release workflow

Add a `repository_dispatch` step at the end of `.github/workflows/release.yml` in this repo (the CLI repo), after the
GoReleaser step. This mirrors the pattern used by the SDK repo to trigger `spec-sync.yml`:

```yaml
      - name: Notify linux-packages repo
        if: success()
        run: |
          gh api repos/supermetrics-public/linux-packages/dispatches \
            -f event_type=release-published \
            -f 'client_payload[tag]=${{ github.ref_name }}'
        env:
          GH_TOKEN: ${{ secrets.LINUX_PACKAGES_TOKEN }}
```

Requires a `LINUX_PACKAGES_TOKEN` secret in the CLI repo — a GitHub PAT with `repo` scope on `linux-packages`.
Same pattern as `HOMEBREW_TAP_TOKEN` for the Homebrew tap and `CLI_DISPATCH_TOKEN` for the SDK→CLI dispatch.

#### 5. User install instructions

**APT (Debian / Ubuntu):**
```bash
# Add signing key
curl -fsSL https://supermetrics-public.github.io/linux-packages/pubkey.gpg \
  | sudo gpg --dearmor -o /usr/share/keyrings/supermetrics.gpg

# Add repository
echo "deb [signed-by=/usr/share/keyrings/supermetrics.gpg] https://supermetrics-public.github.io/linux-packages/ stable main" \
  | sudo tee /etc/apt/sources.list.d/supermetrics.list

# Install
sudo apt-get update && sudo apt-get install supermetrics

# Upgrade (after a new release)
sudo apt-get update && sudo apt-get upgrade supermetrics
```

**YUM / DNF (RHEL / Fedora / Amazon Linux):**
```bash
# Add repository
sudo tee /etc/yum.repos.d/supermetrics.repo <<EOF
[supermetrics]
name=Supermetrics CLI
baseurl=https://supermetrics-public.github.io/linux-packages/yum/
gpgcheck=1
gpgkey=https://supermetrics-public.github.io/linux-packages/pubkey.gpg
enabled=1
EOF

# Install
sudo yum install supermetrics    # or: sudo dnf install supermetrics

# Upgrade (after a new release)
sudo yum update supermetrics
```

**Alpine Linux (apk):** Alpine package repos require a different signing mechanism (`abuild-sign`) and are uncommon
for third-party tools. Use direct install from GitHub Releases (see above).

### Package history and GitHub Pages limits

GitHub Pages has a **1 GB storage** and **100 GB bandwidth/month** cap. With typical CLI binaries (~10 MB per
platform/arch combo, 4 Linux packages per release), this allows roughly 20–25 releases before needing cleanup.

The workflow above does **not** preserve packages from previous releases — each run rebuilds the repo from the
latest release only. This keeps the setup simple and well within Pages limits. To retain multiple versions,
the workflow can be extended to download packages from the N most recent releases using
`gh release list --limit N` before running `reprepro includedeb` for each.

### Secrets needed

| Secret                 | Repo               | Purpose                                           | 1Password entry                      |
|------------------------|--------------------|---------------------------------------------------|--------------------------------------|
| `GPG_PRIVATE_KEY`      | `linux-packages`   | Sign APT and YUM repo metadata                    | `GPG Signing Key — Linux Packages`   |
| `KEY_ID` (variable)    | `linux-packages`   | GPG key fingerprint                               | `GPG Signing Key — Linux Packages`   |
| `LINUX_PACKAGES_TOKEN` | `supermetrics-cli`  | Trigger `repository_dispatch` on `linux-packages` | `GitHub PAT — Linux Packages`        |

Compare with the Homebrew tap setup: `HOMEBREW_TAP_TOKEN` in `supermetrics-cli` serves the same role as
`LINUX_PACKAGES_TOKEN` — a PAT that allows the CLI release workflow to push to / dispatch events on another repo.

**All secrets must be stored in 1Password before being added to GitHub.** GitHub repo secrets cannot be read back
after creation — 1Password is the recovery path if a secret needs to be rotated or re-added.

### Implementation checklist

- [x] Generate GPG signing key (RSA 4096, no passphrase)
- [x] Store GPG private key, public key, and fingerprint in 1Password (`GPG Signing Key — Linux Packages`)
- [x] Create `supermetrics-public/linux-packages` repo (public, empty) — **requires GitHub admin**
- [x] Commit `pubkey.gpg` and `.github/workflows/update-repo.yml` to the repo
- [x] Enable GitHub Pages with GitHub Actions source — **requires GitHub admin**
- [x] Add `GPG_PRIVATE_KEY` secret and `KEY_ID` variable to `linux-packages` repo (copy from 1Password)
- [x] Create `github-pages` environment in `linux-packages` repo
- [x] Create GitHub PAT with `repo` scope on `linux-packages`, store in 1Password (`GitHub PAT — Linux Packages`)
- [x] Add `LINUX_PACKAGES_TOKEN` secret to `supermetrics-cli` repo (copy PAT from 1Password) — **requires GitHub admin**
- [x] Add the `repository_dispatch` step to `.github/workflows/release.yml`
- [x] Delete `private.asc` from local machine (1Password is now the source of truth)
- [ ] Test: run release.yml via workflow_dispatch, verify GoReleaser produces `.deb`/`.rpm`, verify `linux-packages` workflow triggers
- [ ] Test: install `.deb` on Ubuntu via the APT repo, `.rpm` on Fedora via YUM repo
- [ ] Add install instructions to project README

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
  3. Install toolchain via mise (Go, Python, uv, goimports)
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

Requires a `CLI_DISPATCH_TOKEN` secret in the SDK repo (GitHub PAT with `repo` scope on the CLI repo). Store the PAT
in 1Password as `GitHub PAT — CLI Dispatch` before adding it to the SDK repo's secrets.

## Auto-Release Workflow (`.github/workflows/auto-release.yml`)

> **Not implemented.** This section is a design proposal — the workflow file does not exist. Releases are cut
> manually today by running `release.yml` via `workflow_dispatch` with an explicit version.

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
5. A maintainer runs release.yml via workflow_dispatch with the next version (e.g. v0.3.1)
   (once auto-release.yml exists, steps 5-6 become automatic on generated-code changes)
6. release.yml validates the version and pushes the tag
7. GoReleaser builds binaries for 5 platforms, .deb/.rpm/.apk packages
8. GitHub Release published with binaries + checksums + packages
9. Homebrew tap formula auto-updated (via GoReleaser brews section)
10. release.yml sends repository_dispatch to linux-packages repo
11. linux-packages rebuilds APT + YUM repo metadata, deploys to GitHub Pages
12. Users see "new version available" within a week (periodic check)
13. Users run "supermetrics version upgrade" → binary replaced in-place
```

## 1Password Inventory

All CI/CD secrets are stored in 1Password as the source of truth. GitHub repo secrets cannot be read back after
creation — 1Password is the recovery path for rotation or re-provisioning.

Recommended vault: a shared team vault (e.g., "Supermetrics CLI / CI") accessible to engineers with GitHub admin access.

| 1Password entry                      | Type        | Contains                                   | Used by                               |
|--------------------------------------|-------------|--------------------------------------------|-----------------------------------------|
| `GPG Signing Key — Linux Packages`   | Secure Note | Private key (armored), fingerprint, pubkey | `linux-packages` repo secrets           |
| `GitHub PAT — Homebrew Tap`          | Login/Token | Classic PAT, `repo` scope on `homebrew-tap`| `HOMEBREW_TAP_TOKEN` in `supermetrics-cli` |
| `GitHub PAT — Linux Packages`        | Login/Token | Classic PAT, `repo` scope on `linux-packages` | `LINUX_PACKAGES_TOKEN` in `supermetrics-cli` |
| `GitHub PAT — CLI Dispatch`          | Login/Token | Classic PAT, `repo` scope on `supermetrics-cli` | `CLI_DISPATCH_TOKEN` in SDK repo       |

**Rotation procedure**: Generate a new PAT/key → update the 1Password entry → update the GitHub secret → verify the
next release succeeds. PATs should use the minimum required scope (`repo` on the single target repo when possible).

## Implementation Checklist

- [x] Create `.goreleaser.yaml` with builds, archives, checksum, brews, nfpms sections
- [x] Create `.github/workflows/release.yml` (workflow_dispatch with a version input, tags and runs GoReleaser)
- [x] Create `.github/workflows/spec-sync.yml` (repository_dispatch handler)
- [ ] Create `.github/workflows/auto-release.yml` (tag bumper on generated code changes)
- [ ] Create `supermetrics-public/homebrew-tap` repo — **requires GitHub admin** (see setup instructions above)
- [ ] Create `supermetrics-public/linux-packages` repo — **requires GitHub admin** (see setup instructions above)
- [ ] Generate GPG signing key, store in 1Password (`GPG Signing Key — Linux Packages`)
- [ ] Create GitHub PATs, store in 1Password, add as repo secrets:
  - `HOMEBREW_TAP_TOKEN` in `supermetrics-cli` → 1Password `GitHub PAT — Homebrew Tap`
  - `LINUX_PACKAGES_TOKEN` in `supermetrics-cli` → 1Password `GitHub PAT — Linux Packages`
  - `CLI_DISPATCH_TOKEN` in SDK repo → 1Password `GitHub PAT — CLI Dispatch`
- [ ] Add dispatch step to SDK repo's CI workflow — **requires SDK repo access**
- [ ] Test: run release.yml via workflow_dispatch, verify GoReleaser produces correct artifacts
- [ ] Test: verify Homebrew tap formula is updated
- [ ] Test: verify `linux-packages` workflow triggers and APT/YUM repos are published
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
| CLI bug fix (non-generated code)            | Not auto-released                | Patch         | Run release.yml via workflow_dispatch with the new version    |

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

3. **Infrastructure not yet provisioned.** The Homebrew tap repo (`supermetrics-public/homebrew-tap`), Linux packages
   repo (`supermetrics-public/linux-packages`), repo secrets (`HOMEBREW_TAP_TOKEN`, `LINUX_PACKAGES_TOKEN`,
   `CLI_DISPATCH_TOKEN`), GPG signing key, and the SDK-side dispatch step all require GitHub admin setup. See the
   implementation checklist and repo setup instructions above.
