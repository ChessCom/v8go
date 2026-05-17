# Maintaining github.com/ChessCom/v8go

This document tracks the one-time setup needed to make the CI workflows
in `.github/workflows/` actually run end to end, plus the branch
protection rules that gate `main`.

## Required repository secrets

| Secret | Used by | Scope |
|---|---|---|
| `CROSS_REPO_READ_TOKEN` | `compat-blindfox`, `compat-er` in `ci.yml` | `repo:read` on `ChessCom/blindfox` and `ChessCom/er` |
| `CROSS_REPO_WRITE_TOKEN` | `auto-bump-downstreams.yml` | `repo:write` (PR open) on `ChessCom/blindfox` and `ChessCom/er` |

Both should be fine-scoped GitHub App tokens (preferred) or PATs with
the minimum scopes above. The workflow jobs auto-skip when the secret
is missing, so PR runs from forks degrade gracefully instead of
failing.

To rotate, create a new token with the same scopes, paste it into
**Settings → Secrets and variables → Actions** under the matching
name, and remove the old one.

## Required status checks (branch protection on `main`)

Configure under **Settings → Branches → main → Branch protection
rules** with the following selections:

* Require a pull request before merging
* Require approvals: 1
* Dismiss stale pull request approvals when new commits are pushed
* Require review from Code Owners (uses `CODEOWNERS`)
* Require status checks to pass before merging:
  * `unit (ubuntu-latest)`
  * `unit (macos-latest)`
  * `vet (gofmt + go vet + clang-format)`
  * `compat-blindfox (ubuntu-latest)`
  * `compat-blindfox (macos-latest)`
  * `compat-er (ubuntu-latest)`
  * `compat-er (macos-latest)`
* Require branches to be up to date before merging
* Restrict who can push to matching branches: maintainers only

`compat-blindfox` and `compat-er` only run when
`CROSS_REPO_READ_TOKEN` is configured. Until that secret lands, mark
those checks as **non-required** (otherwise PRs from forks will be
unmergeable) and flip them to required once the token is in place.

## Release cadence

Tags follow `vMAJOR.MINOR.PATCH-chess.N` (e.g. `v0.34.0-chess.0`),
where `MAJOR.MINOR.PATCH` mirrors the upstream `tommie/v8go` version
the fork tracks and `chess.N` increments per ChessCom-side change set.
The first published tag is `v0.34.0-chess.0`.

When cutting a release:

1. Update `CHANGELOG.md` with a new section.
2. `git tag vX.Y.Z-chess.N` and push the tag.
3. Trigger the **auto-bump-downstreams** workflow manually to open PRs
   in `blindfox` and `er` against the new tag SHA. (Once the
   downstreams have migrated off `github.com/tommie/v8go`, flip that
   workflow's `on` trigger from `workflow_dispatch` to `push: branches:
   [main]` for automatic bumps.)
