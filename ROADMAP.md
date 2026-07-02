# Roadmap

Tracking the build of `pr-tree`. See the full design in
[`docs/superpowers/specs/2026-06-08-pr-tree-design.md`](docs/superpowers/specs/2026-06-08-pr-tree-design.md).

## Status

| Command | State | Notes |
|---|---|---|
| `list` | ✅ Done | Read-only tree rendering with `--mine` / `--to-review`. First build. |
| `annotate` | ✅ Done | Upsert a marker-wrapped links block (`upstream`/`downstream`) into every PR in a tree; dry-run with a coloured diff, `--apply` to write. |
| `replant` | ✅ Done | Dry-run plan + `--apply` (rebase, idempotent resume, `--force-with-lease` push, `--re-request-reviews`). |

## Foundations (shared, built alongside `list`)

- [x] `internal/config` — repo detection from git `origin`, `--repo` override.
- [x] `internal/github` — token discovery (`gh auth token` → env), GraphQL client, PR fetch.
- [x] `internal/tree` — forest building (base→head topology), `links:` parsing, filter/prune.
- [x] `internal/render` — forest → text with connectors and markers.
- [x] `cmd/pr-tree` — cobra CLI wiring.

## `list` — first build

- [x] Fetch open PRs (OPEN + DRAFT) via GraphQL; best-effort merged/closed (`links:`-referenced parents).
- [x] Build forest; place merged nodes via `links:` (documented limitation).
- [x] `--mine`, `--to-review` (with `<== Review pending`), default (all), union.
- [x] Render matching the README format.
- [x] Unit tests for `tree` / `render`; mocked test for `github`.

## `annotate` — done

- [x] Design: [`docs/superpowers/specs/2026-07-02-annotate-design.md`](docs/superpowers/specs/2026-07-02-annotate-design.md).
- [x] Marker-wrapped links block; immediate upstream/downstream; parseable by
  `tree.ParseUpstream`.
- [x] Idempotent upsert (`internal/annotate`): replace prior pr-tree block in
  place, strip free-form "stacked on" notes.
- [x] Coloured unified diff of the description (`internal/render`).
- [x] `updatePullRequest` mutation (`internal/github`).
- [x] `annotate [#PR]` dry-run + `--apply` behind a `[y/N]` prompt (`-y` skips).

## `replant` — last

- [x] Strategy: drop redundant commits **structurally** via
  `git rebase --onto <newbase> <fork-point> <child>`, where
  `fork-point = git merge-base <parent-head-OID> <child-head>`. Patch-id dedup
  (`git cherry`, plain `rebase`) is unreliable under squash-merge.
- [x] Fetch `headRefOid` for every PR (retained by GitHub even after merge).
- [x] Pure planning engine (`internal/replant`) + git wrappers (`internal/git`).
- [x] `replant [#PR]` dry-run: infers the target from the current branch, prints
  per-descendant drop/keep commits.
- [x] **Execute**: run the rebases + `--force-with-lease` push, behind a
  confirmation prompt. On conflict, pause and let the user resolve, then resume.
  NOTE: must run rebase with `-c core.commentChar=#` — a global
  `rebase.updateRefs=true` injects `# Ref …` todo lines that break under a
  non-`#` `core.commentChar`.
  - `--apply`: triggers the rebase + push phase (dry-run by default without it).
  - Idempotent resume: re-running after resolving a conflict skips branches
    already successfully replanted in a prior run.
  - `--re-request-reviews`: after force-push, re-requests review from each
    pushed PR's approvers whose approval was staled by the force-push; in
    dry-run mode it lists who would be re-requested.
