# Roadmap

Tracking the build of `pr-tree`. See the full design in
[`docs/superpowers/specs/2026-06-08-pr-tree-design.md`](docs/superpowers/specs/2026-06-08-pr-tree-design.md).

## Status

| Command | State | Notes |
|---|---|---|
| `list` | 🚧 In progress | Read-only tree rendering with `--mine` / `--to-review`. First build. |
| `annotate` | ⬜ Planned | Upsert a `links:` (`upstream`/`downstream`) section into PR descriptions. |
| `replant` | ⬜ Planned | Rebase + force-push all descendants of a PR. Risky; careful design needed. |

## Foundations (shared, built alongside `list`)

- [ ] `internal/config` — repo detection from git `origin`, `--repo` override.
- [ ] `internal/github` — token discovery (`gh auth token` → env), GraphQL client, PR fetch.
- [ ] `internal/tree` — forest building (base→head topology), `links:` parsing, filter/prune.
- [ ] `internal/render` — forest → text with connectors and markers.
- [ ] `cmd/pr-tree` — cobra CLI wiring.

## `list` — first build

- [ ] Fetch open PRs (OPEN + DRAFT) via GraphQL; best-effort merged/closed.
- [ ] Build forest; place merged nodes via `links:` (documented limitation).
- [ ] `--mine`, `--to-review` (with `<== Review pending`), default (all), union.
- [ ] Render matching the README format.
- [ ] Unit tests for `tree` / `render`; mocked test for `github`.

## `annotate` — next

- [ ] Design (idempotent upsert, link format, dry-run, scope of tree to touch).

## `replant` — last

- [ ] Design (rebase order, conflict handling, force-push safety, current-PR inference).
