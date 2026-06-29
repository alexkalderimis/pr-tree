# AGENTS.md

Guidance for AI coding agents working in this repository. For the full
developer guide (structure, architecture, testing philosophy), see
[`DEVELOPMENT.md`](DEVELOPMENT.md).

## What this is

`pr-tree` is a Go CLI that visualizes stacks of GitHub pull requests as trees.
It is built **one command at a time** — see [`ROADMAP.md`](ROADMAP.md) for what
is done and what is next (`list` → `annotate` → `replant`).

## Essential commands

```sh
go build ./...                                   # build
go test ./...                                    # run all tests (or: make test)
go vet ./...                                      # vet
gofmt -l .                                        # must print nothing
go run ./cmd/pr-tree list --repo owner/name      # run (needs a GitHub token)
```

A `Makefile` provides `make pr-tree` (build), `make test`, `make install`, and
`make clean`.

A GitHub token is discovered from `gh auth token`, then `GH_TOKEN`, then
`GITHUB_TOKEN`. Pure-logic tests need no token.

## Where things live

| Package | Responsibility |
|---|---|
| `cmd/pr-tree/` | cobra CLI wiring + subcommands |
| `internal/config/` | repo resolution (`--repo` or git `origin`) |
| `internal/github/` | token discovery + GraphQL client |
| `internal/git/` | local `git` command wrappers (merge-base, rev-list, …) |
| `internal/tree/` | **pure** forest building, filtering, link parsing |
| `internal/replant/` | **pure** rebase planning (descendant order + new bases) |
| `internal/render/` | **pure** tree rendering |

## Conventions

- **Test-driven, always.** Write the failing test first, run it to confirm it
  fails, then implement. Each `*.go` has a `*_test.go` beside it.
- **Keep the pure/I/O boundary.** `tree`, `replant`, and `render` must stay free
  of network and git access — they take data in and return data out. Put I/O in
  `github`, `config`, and `git`, and test the network ones with `httptest` /
  injected helpers (no real network or env in tests). **Exception:** `internal/git`
  wraps the `git` binary, which can only be tested honestly by running it; its
  tests build throwaway repos in `t.TempDir()`. The *planning* logic stays pure
  and mock-free.
- **Small, focused files**, one clear responsibility each.
- **Keep `ROADMAP.md` current** when a command or foundation lands.
- **Don't commit build artifacts** — the `pr-tree` binary is git-ignored.
- **Commit messages**: imperative subject, a short body when the change isn't
  obvious. Existing history ends commit messages with:
  `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`

## Before starting non-trivial work

Read the design spec and relevant plan under
[`docs/superpowers/`](docs/superpowers/). The spec records decisions (the
branch-topology tree model, the `links:`-based reconstruction of merged nodes
and its documented limitation) — honor them or update the spec deliberately.

## Definition of done

`gofmt -l .` is empty, `go vet ./...` is clean, `go test ./...` passes, and
`ROADMAP.md` reflects reality.
