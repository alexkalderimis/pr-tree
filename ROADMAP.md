# Roadmap

Tracking the build of `pr-tree`. See the full design in
[`docs/superpowers/specs/2026-06-08-pr-tree-design.md`](docs/superpowers/specs/2026-06-08-pr-tree-design.md).

## Status

| Command | State | Notes |
|---|---|---|
| `list` | ✅ Done | Read-only tree rendering with `--mine` / `--to-review`. First build. |
| `annotate` | ⬜ Planned | Upsert a `links:` (`upstream`/`downstream`) section into PR descriptions. |
| `replant` | 🟡 Dry-run | Prints the rebase plan (drop/keep commits per descendant). Execute + force-push still pending. |

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

## `annotate` — next

- [ ] Design (idempotent upsert, link format, dry-run, scope of tree to touch).

## `replant` — last

- [x] Strategy: drop redundant commits **structurally** via
  `git rebase --onto <newbase> <fork-point> <child>`, where
  `fork-point = git merge-base <parent-head-OID> <child-head>`. Patch-id dedup
  (`git cherry`, plain `rebase`) is unreliable under squash-merge.
- [x] Fetch `headRefOid` for every PR (retained by GitHub even after merge).
- [x] Pure planning engine (`internal/replant`) + git wrappers (`internal/git`).
- [x] `replant [#PR]` dry-run: infers the target from the current branch, prints
  per-descendant drop/keep commits.
- [ ] **Execute**: run the rebases + `--force-with-lease` push, behind a
  confirmation prompt. On conflict, pause and let the user resolve, then resume.
  NOTE: must run rebase with `-c core.commentChar=#` — a global
  `rebase.updateRefs=true` injects `# Ref …` todo lines that break under a
  non-`#` `core.commentChar`.
