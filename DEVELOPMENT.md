# Development

How to build, test, and work on `pr-tree`.

## Prerequisites

- **Go 1.21+** (developed against 1.21.1).
- **A GitHub token** for anything that hits the API. `pr-tree` discovers one
  from, in order:
  1. `gh auth token` (the [GitHub CLI](https://cli.github.com/) — recommended;
     run `gh auth login` once),
  2. the `GH_TOKEN` environment variable,
  3. the `GITHUB_TOKEN` environment variable.

  Pure-logic tests do not need a token; only running the CLI against a real
  repo does.

## Common commands

| Task | Command |
|---|---|
| Build all packages | `go build ./...` |
| Build the binary | `go build -o pr-tree ./cmd/pr-tree` |
| Run without building | `go run ./cmd/pr-tree list --repo owner/name` |
| Run all tests | `go test ./...` |
| Run one package's tests, verbose | `go test ./internal/tree/ -v` |
| Run a single test | `go test ./internal/tree/ -run TestBuildForest -v` |
| Vet | `go vet ./...` |
| Check formatting | `gofmt -l .` (prints files needing formatting; should be empty) |
| Apply formatting | `gofmt -w .` |
| Tidy dependencies | `go mod tidy` |

Before committing, the working expectation is: `gofmt -l .` is empty,
`go vet ./...` is clean, and `go test ./...` passes.

### Make shortcuts

A `Makefile` wraps the most common tasks:

| Command | Does |
|---|---|
| `make` / `make pr-tree` | Build the `pr-tree` binary (rebuilds only when sources change) |
| `make test` | `go test ./...` |
| `make install` | `go install ./cmd/pr-tree` (into `go env GOBIN`, else `GOPATH/bin`) |
| `make clean` | Remove the locally built binary |

## Repository structure

Pure logic is deliberately separated from I/O so the interesting parts are
unit-testable without network or git.

| Path | Responsibility | I/O? |
|---|---|---|
| `cmd/pr-tree/` | CLI entry point and cobra wiring (`main`, `root`, `list`, `replant`) | yes (glue) |
| `internal/config/` | Resolve the target repo from `--repo` or the git `origin` remote | git |
| `internal/github/` | Token discovery and the GitHub GraphQL client (`FetchOpenPRs`, `FetchPRsByNumber`, `Viewer`) | network |
| `internal/git/` | Local `git` command wrappers used by `replant` (`MergeBase`, `RevList`, `CurrentBranch`, …) | git |
| `internal/tree/` | Domain model + forest building (branch topology with `upstream:` link fallback), filtering, selection | none (pure) |
| `internal/replant/` | Pure rebase planning: descendant order and new-base selection for `replant` | none (pure) |
| `internal/render/` | Render a forest to the textual tree output | none (pure) |
| `docs/superpowers/` | Design spec and implementation plans | — |

Each `*.go` file has a focused `*_test.go` beside it.

### How a `list` invocation flows

1. `config.Resolve` determines the repo (`--repo owner/name`, else git `origin`).
2. `github.Token` discovers a token; `github.New` builds a client.
3. `client.Viewer` gets the authenticated login; `client.FetchOpenPRs` pulls
   open PRs and the default branch.
4. Merged/closed parents referenced by `upstream:` links but missing from the
   open set are fetched best-effort (`FetchPRsByNumber`).
5. `tree.BuildForest` assembles the forest; `tree.SelectTrees` filters by
   `--mine` / `--to-review`; `tree.ReviewPending` marks pending reviews.
6. `render.Render` writes the tree to stdout.

## Testing philosophy

- **Test-driven.** Write the failing test first, watch it fail, then implement.
- **Pure packages (`tree`, `render`) get table-driven unit tests** over fixtures
  — no network, no git.
- **I/O packages are tested without real I/O:** `github` uses `httptest`
  servers and decodes canned GraphQL responses; `config` and the token logic
  use injected helpers (e.g. a `getenv func(string) string`) rather than
  touching the real environment. The one exception is `internal/git`, which
  wraps the `git` binary and can only be tested honestly by running it — its
  tests build throwaway repositories in `t.TempDir()`.
- Tests assert real behavior (exact rendered strings, forest shape), not mocks
  echoing themselves.

## Project docs

- [`README.md`](README.md) — what the tool is and how to use it.
- [`ROADMAP.md`](ROADMAP.md) — build status across `list` → `annotate` →
  `replant`. **Keep it current** as commands land.
- [`docs/superpowers/specs/`](docs/superpowers/specs/) — the design spec
  (decisions, tree model, the merged-node limitation).
- [`docs/superpowers/plans/`](docs/superpowers/plans/) — task-by-task
  implementation plans.

## Adding the next command

`pr-tree` is built one command at a time (see `ROADMAP.md`). The pattern:

1. Brainstorm/spec the command — `annotate`'s `links:` format and `replant`'s
   rebase safety have real design choices worth settling first.
2. Add a focused implementation plan under `docs/superpowers/plans/`.
3. Implement test-first, reusing the pure `tree`/`render` core and the
   `github`/`config` packages. Add the subcommand under `cmd/pr-tree/`.
4. Update `ROADMAP.md`.

See [`AGENTS.md`](AGENTS.md) for conventions when working with AI coding agents.
