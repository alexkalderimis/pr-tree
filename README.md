# PR tree-visualizer

Visualize GitHub pull-requests as a tree, the way nature intended.

## The problem

You have a lot of open PRs, either authored or in-review, and they are not all into the default branch, but
form complex stacks of PRs. Keeping track of them is difficult!

This tool helps you view and manage these stacks

## Usage

List your PRs:

```sh
> pr-tree list --mine # List your PRs:

#1234 (ROOT, reviewer: @abc, MERGED)
└ #1235 (STEM, reviewer: @xyz, OPEN)
  └ #1236 (LEAF-1, reviewer: @foo, OPEN)
  └ #1237 (LEAF-2, reviewer: @bar, DRAFT)
#1240 (ANOTHER ROOT, reviewer: @abc, OPEN)
```

List PRs that are in your review queue, including the context of other PRs in the tree:

```sh
> pr-tree list --to-review

#1234 (A, reviewer: @foo, OPEN)
└ #1235 (B, OPEN) <== Review pending
  └ #1236 (C, reviewer: @bar, OPEN)
    └ #1237 (D, reviewer: @bar, DRAFT)
#1240 (Another one, OPEN) <== Review pending
```

Other commands:

- `replant` - rebase all descendants (infers the current PR from the current branch, accepts `[#PR_ID]` argument)
- `annotate` - update the PR descriptions of all PRs in the tree with a `links:` section including `upstream` and `downstream` links, so that the stack is navigable within GitHub

> **Note:** `list` is implemented today; `annotate` and `replant` are planned. See [ROADMAP.md](ROADMAP.md).

### Color

By default, `list` colors its output when writing to a terminal — titles are bold, and PR numbers, reviewers, status, and the review marker are colored to make dense forests easier to scan. Color is disabled automatically when output is piped or redirected. To force it off, pass `--no-color` or set the `NO_COLOR` environment variable.

A trailing green `✓` marks a PR that has received the reviews required to merge (GitHub's review decision is *approved*). The `✓` is shown without color when color is disabled.

## Development

- [DEVELOPMENT.md](DEVELOPMENT.md) — repository structure, build/test/run commands, and architecture.
- [AGENTS.md](AGENTS.md) — conventions for AI coding agents.
- [ROADMAP.md](ROADMAP.md) — build status across `list` → `annotate` → `replant`.

