# GNS keys and triggers for ChessCom/v8go

This document is the authoritative registry of all GNS keys and
triggers associated with the v8go repository. All keys live under
`user.yuri.public.v8go` and triggers under `user.yuri.triggers.v8go-*`.

## Knowledge keys

| Key | Description | Managed by |
|---|---|---|
| `user.yuri.public.v8go.bootstrap` | Repo operational reference: build, test, architecture, data slots, concurrency model, snapshot system | Manual update after API surface changes |
| `user.yuri.public.v8go.code-review` | Adversarial review procedure: C++ audit (C1-C9), Go audit (G1-G7), test coverage (T1-T6), performance gates, architecture drift guards | Manual update when new audit checks are added |
| `user.yuri.public.v8go.benchmarks.latest` | Latest weekly benchmark baseline (ns/op, B/op, allocs/op for cold-start, speedup ratios) | `v8go-benchmark` trigger |
| `user.yuri.public.v8go.maintenance-log` | Append-only log of maintenance ceremony executions | All maintenance triggers |

### Reading keys

```bash
gns get user.yuri.public.v8go.bootstrap
gns get user.yuri.public.v8go.code-review
gns get user.yuri.public.v8go.benchmarks.latest
gns get user.yuri.public.v8go.maintenance-log
```

### Updating keys

```bash
# Bootstrap (after API surface change)
gns set user.yuri.public.v8go.bootstrap --stdin < docs/bootstrap.md

# Code review (after adding new audit checks)
gns set user.yuri.public.v8go.code-review --stdin < docs/code-review.md
```

## Triggers

| Trigger key | Type | Schedule | Description |
|---|---|---|---|
| `user.yuri.triggers.v8go-pr-review` | setupAgent (pipe) | On PR open/reopen | Adversarial code review on every PR |
| `user.yuri.triggers.v8go-benchmark` | setupAgent (cron) | Sunday 08:00 UTC | Weekly benchmark baseline collection |
| `user.yuri.triggers.v8go-upstream-check` | setupAgent (cron) | Sunday 09:00 UTC | Weekly upstream sync PR triage |
| `user.yuri.triggers.v8go-maintenance` | setupAgent (API) | On-demand | Full maintenance ceremony (benchmarks + docs + upstream + compat) |

### Trigger bootstraps

Each setupAgent trigger has a corresponding bootstrap at
`user.yuri.trigger-bootstraps.v8go-*` that defines the agent's
identity, loaded GNS keys, prompt template, and required scopes.

| Bootstrap key | Loads | Scopes |
|---|---|---|
| `user.yuri.trigger-bootstraps.v8go-pr-review` | bootstrap, code-review | GitHub contents:read, pull_requests:write on ChessCom/v8go |
| `user.yuri.trigger-bootstraps.v8go-benchmark` | bootstrap | GNS read/write on user.yuri.public.v8go.* |
| `user.yuri.trigger-bootstraps.v8go-upstream-check` | bootstrap | GitHub contents:read, pull_requests:read on ChessCom/v8go |
| `user.yuri.trigger-bootstraps.v8go-maintenance` | bootstrap, code-review | GitHub contents:read, pull_requests:read; GNS read/write |

### Invoking triggers manually

```bash
# On-demand maintenance
gns triggers invoke user.yuri.triggers.v8go-maintenance

# Force a benchmark run
gns triggers invoke user.yuri.triggers.v8go-benchmark

# Force an upstream check
gns triggers invoke user.yuri.triggers.v8go-upstream-check
```

### Checking trigger status

```bash
gns triggers status user.yuri.triggers.v8go-benchmark
gns triggers status user.yuri.triggers.v8go-upstream-check
gns triggers status user.yuri.triggers.v8go-maintenance
```

## Automation flow

```
Sunday 08:00 UTC ──► v8go-benchmark trigger
                     ├── Run benchmarks locally
                     ├── Store baseline in GNS
                     └── Notify on regression

Sunday 09:00 UTC ──► v8go-upstream-check trigger
                     ├── Check for open sync PRs
                     ├── Triage CI status
                     ├── Check V8 version changes
                     └── Notify if action needed

On PR open ────────► v8go-pr-review trigger
                     ├── Load bootstrap + code-review
                     ├── Fetch PR diff
                     ├── Run adversarial review
                     └── Post review via GitHub API

On demand ─────────► v8go-maintenance trigger
                     ├── Run all ceremonies (A-D)
                     ├── Update docs if needed
                     └── Log to maintenance-log
```
