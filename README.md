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

`replant` is **dry-run only** today: it prints, for each descendant, which commits would be dropped and which kept, but performs no rebase or force-push.

```sh
> pr-tree replant 1234   # or, on the branch itself: pr-tree replant

Replant plan for #1234 (ROOT) — merged — moving children onto main:

  #1235 (STEM) → rebase onto main (was feature-a)
      drop 3 commits  a1f2c3d..c4d5e6f  (merged via #1234)
      keep 2 commits  b1f9e0a add parser

(dry-run: no branches were rebased or pushed — execution is not yet implemented)
```

After a squash-merge, the redundant parent commits are identified **structurally** — `replant` rebases each child with `git rebase --onto <new-base> <fork-point> <child>`, where the fork point is `git merge-base` of the parent's recorded head (GitHub's `headRefOid`) and the child's head. This drops exactly the merged commits regardless of squashing, where patch-id detection (`git cherry`) would not.

> **Note:** `list` is implemented today; `replant` is dry-run only; `annotate` is planned. See [ROADMAP.md](ROADMAP.md).

### Color

By default, `list` colors its output when writing to a terminal — titles are bold, and PR numbers, reviewers, status, and the review marker are colored to make dense forests easier to scan. Color is disabled automatically when output is piped or redirected. To force it off, pass `--no-color` or set the `NO_COLOR` environment variable.

A trailing green `✓` marks a PR that has received the reviews required to merge (GitHub's review decision is *approved*), and the line itself is rendered in bold so approved PRs stand out when scanning a stack. The `✓` is shown without color, and the bold is dropped, when color is disabled.

## Development

- [DEVELOPMENT.md](DEVELOPMENT.md) — repository structure, build/test/run commands, and architecture.
- [AGENTS.md](AGENTS.md) — conventions for AI coding agents.
- [ROADMAP.md](ROADMAP.md) — build status across `list` → `annotate` → `replant`.

