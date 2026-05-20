# Releasing ChessCom/v8go

This document describes how to cut a release of the ChessCom/v8go fork,
how consumers integrate it, and the automation behind the process.

## Versioning scheme

Tags follow `vMAJOR.MINOR.PATCH-chess.N`:

| Segment | Meaning |
|---|---|
| `MAJOR.MINOR.PATCH` | Mirrors the upstream `tommie/v8go` version this fork tracks |
| `chess.N` | Increments for each ChessCom-side change set on that upstream base |

Examples:

- `v0.34.0-chess.0` -- first release on top of upstream `v0.34.0`
- `v0.34.0-chess.1` -- second change set, same upstream base
- `v0.35.0-chess.0` -- first release after syncing with upstream `v0.35.0`

## When to release

Cut a release when:

- A PR merges to `main` that adds features, fixes bugs, or changes API
  surface and the change is ready for downstream consumption.
- An upstream sync merges that bumps the V8 version or changes
  behaviour downstream consumers should pick up.

Not every merge requires a release. Documentation-only or CI-only
changes can be batched into the next feature release.

## How to release

### 1. Update the changelog

Add a new section at the top of `CHANGELOG.md` with the tag you are
about to create:

```markdown
## v0.34.0-chess.2 -- 2026-06

### Added
- ...

### Changed
- ...
```

Commit this change to `main` (directly or via PR).

### 2. Tag and push

```bash
git tag v0.34.0-chess.2
git push origin v0.34.0-chess.2
```

### 3. Automation takes over

Pushing the tag triggers the
[release workflow](../.github/workflows/release.yml), which:

1. **Validates** the tag matches `vX.Y.Z-chess.N`.
2. **Runs the full CI suite** (build, test, coverage on ubuntu + macOS).
3. **Creates a GitHub Release** with release notes extracted from the
   matching `CHANGELOG.md` section.

### 4. Verify

- Check the [Releases page](https://github.com/ChessCom/v8go/releases)
  for the new release.
- Notify downstream consumers (blindfox, er) to update their
  `go.mod` to the new tag.

## Consumer integration

### Direct dependency (recommended)

```bash
go get github.com/ChessCom/v8go@v0.34.0-chess.2
```

This works once the tag is pushed. Go modules resolve the tag to a
specific commit and lock it in `go.sum`.

### Via replace directive (transitional)

If your project still imports `github.com/tommie/v8go`, use a replace
directive until you can migrate:

```bash
go mod edit -replace="github.com/tommie/v8go=github.com/ChessCom/v8go@v0.34.0-chess.2"
go mod tidy
```

### Pinning to a commit (pre-release)

For unreleased changes on `main`:

```bash
go get github.com/ChessCom/v8go@main
```

Go will resolve this to a pseudo-version like
`v0.0.0-20260517120000-abcdef123456`.

## Hotfix releases

To ship a fix without waiting for the next feature release:

1. Merge the fix to `main`.
2. Increment `chess.N` (e.g. `v0.34.0-chess.2` -> `v0.34.0-chess.3`).
3. Follow the standard tag-and-push flow above.

## Upstream version bumps

When upstream `tommie/v8go` releases a new version (e.g. `v0.35.0`):

1. Merge the upstream sync PR (opened weekly by
   [upstream-sync.yml](../.github/workflows/upstream-sync.yml)).
2. Reset `chess.N` to 0 and use the new upstream version:
   `v0.35.0-chess.0`.
3. Update `CHANGELOG.md` noting the upstream version bump.
4. Tag and push.

## Workflow details

### Workflows

| Workflow | Trigger | What it does |
|---|---|---|
| [`release.yml`](../.github/workflows/release.yml) | Tags matching `v*` | Validates tag format, runs CI, creates GitHub Release |
| [`ci.yml`](../.github/workflows/ci.yml) | Push/PR to `main` | Lint, build, test, coverage, downstream compat |
| [`upstream-sync.yml`](../.github/workflows/upstream-sync.yml) | Weekly (Mon 06:00 UTC) | Fetches upstream, opens merge PR |
| [`build-v8-deps.yml`](../.github/workflows/build-v8-deps.yml) | Manual dispatch | Rebuilds V8 monolith for all platforms (not part of release flow) |

### release.yml jobs

| Job | What it does |
|---|---|
| `validate` | Checks tag format against `v[0-9]+.[0-9]+.[0-9]+-chess.[0-9]+` |
| `ci` | Builds and tests on ubuntu-latest + macos-latest, enforces coverage |
| `release` | Extracts CHANGELOG section, creates GitHub Release |

The `GITHUB_TOKEN` (automatic) is sufficient for creating the GitHub
Release.
