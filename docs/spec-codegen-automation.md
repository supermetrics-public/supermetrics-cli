# Spec Codegen Automation

This repository regenerates its CLI commands from `openapi-spec.yaml`. When the upstream spec
changes, the regeneration (and the tests and docs that go with it) is driven by Claude Code running
in GitHub Actions, so a spec bump doesn't require a human to run `make generate` and hand-fix the
fallout.

## What triggers it

The workflow [`/.github/workflows/spec-codegen.yml`](../.github/workflows/spec-codegen.yml) runs when
**both** conditions hold on a pull request:

- the PR changes `openapi-spec.yaml` (path filter), and
- the PR carries the **`openapi-spec-update`** label (job `if:` guard).

The existing spec-sync flow (see [release-automation.md](release-automation.md)) already opens such a
PR and applies that label, so this workflow slots in on top of it.

## What it does

Claude Code runs the full pipeline on the PR branch and commits the result back to it:

1. `make generate` — regenerate `cmd/generated/` from the spec + `scripts/command-mapping.yaml`. If
   the generator itself breaks on the new spec shape, Claude fixes the generator in `scripts/` and
   re-runs.
2. `make lint`, `make build`, `make test`.
3. Updates/adds the hand-written Go tests in `cmd/generated/*_test.go` to match changed URLs,
   parameters, or new commands.
4. Updates the command tables in `README.md` and adds examples to `docs/examples.md`.
5. Commits everything to the PR branch and posts a summary comment.

**Convention:** the generated Go under `cmd/generated/` is produced by this automation, not
hand-committed. `make generate` is also run in [`ci.yml`](../.github/workflows/ci.yml) on every build,
so the generated tree is always rebuilt from the spec + mapping before lint/build/test.

## Model provider

The workflow uses the official [`anthropics/claude-code-action`](https://github.com/anthropics/claude-code-action)
configured for **Google Vertex AI**. Providing a `prompt:` puts the action in automation mode (it runs
the pipeline immediately, without an `@claude` mention).

### Required for Vertex AI

| Secret / variable | Purpose |
| --- | --- |
| `secrets.GCP_WORKLOAD_IDENTITY_PROVIDER` | Workload Identity Federation provider resource name (OIDC, no static key). |
| `secrets.GCP_SERVICE_ACCOUNT` | Service account to impersonate; needs `roles/aiplatform.user`. |
| `vars.VERTEX_REGION` (optional) | Vertex region, e.g. `us-east5` (defaults in the workflow). |

The `id-token: write` permission is required for WIF. The Vertex model id uses the `model@date` form
(e.g. `claude-sonnet-4-5@20250929`); adjust `--model` in `claude_args` to taste.

### Alternative: LiteLLM / OpenAI-compatible gateway

The workflow includes a commented block for routing through a LiteLLM proxy instead of Vertex. In that
mode you drop the Vertex auth step and `use_vertex`, set `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN`
(Bearer token) in `env:`, and also pass the same token as the `anthropic_api_key` input — the input is
only there to satisfy the action's launch check; the `env` var is what actually authenticates. The
model name is whatever the gateway registers (typically the plain `claude-sonnet-4-5-20250929` form).

### Optional: re-triggering CI on Claude's commits

Commits pushed with the default `GITHUB_TOKEN` do not start new workflow runs. To have CI re-run after
Claude commits, add a GitHub App token via `actions/create-github-app-token` (secrets `APP_ID` and
`APP_PRIVATE_KEY`) and pass it as the action's `github_token` input. Both are wired up as commented
steps in the workflow.

## Iterating on the automation

Because `pull_request` workflows run the workflow file from the PR's head branch, you can iterate on
this workflow (and the generator) directly on a labeled spec-update PR: push changes to the workflow or
`scripts/`, and the next `synchronize` event re-runs the pipeline.
