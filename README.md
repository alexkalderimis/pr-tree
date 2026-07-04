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

Filter the listing further:

- `--root` — only root PRs (those with no unmerged parent), shown as a flat list
- `--parent PRNO` — the sub-tree descending from a PR (empty `PRNO` = the current branch's PR)
- `--tree PRNO` — the whole tree a PR belongs to, ancestors and descendants (empty `PRNO` = the current branch's PR)
- `--approved` — only approved PRs
- `--active` — only non-draft PRs

`--root`, `--parent`, and `--tree` are mutually exclusive. `--approved` and `--active` narrow whichever set is selected, pruning non-matching PRs and sliding survivors up to their nearest surviving ancestor.

Other commands:

- `replant` - rebase all descendants (infers the current PR from the current branch, accepts `[#PR_ID]` argument)
- `annotate` - upsert a machine-managed links block (immediate `upstream`/`downstream` PRs) into the description of every PR in a tree, so the stack is navigable within GitHub
- `go [up|down|root|leaf]` - check out a branch by its position relative to the current PR: `up` to the parent (or the default branch if there is no live parent), `down` to a child, `root` to the nearest unmerged root, `leaf` to the end of the sequence. Prompts when `down`/`leaf` is ambiguous.

`annotate` is **dry-run by default**: it prints a coloured diff of each PR
description it would change and writes nothing unless you pass `--apply`. The
target defaults to the current branch's PR, or pass `[#PR]`; the whole tree the
target belongs to is annotated. Existing "Stacked on #N" notes and prior
pr-tree blocks are replaced, so re-running is idempotent.

Flags:

- `--apply` — write the descriptions (omit for dry-run).
- `-y` / `--yes` — skip the `Update N PR description(s)? [y/N]` confirmation.
- `--no-color` — disable coloured output.

`replant` is **dry-run by default**: it prints, for each descendant, which commits would be dropped and which kept, but performs no rebase or force-push unless you pass `--apply`.

```sh
> pr-tree replant 1234   # or, on the branch itself: pr-tree replant

Replant plan for #1234 (ROOT) — merged — moving children onto main:

  #1235 (STEM) → rebase onto main (was feature-a)
      drop 3 commits  a1f2c3d..c4d5e6f  (merged via #1234)
      keep 2 commits  b1f9e0a add parser

(dry-run: no branches were rebased or pushed — pass --apply to execute)
```

Pass `--apply` to actually run the rebases and force-push. `replant` rebases every descendant top-down, then — only if all rebases succeed — asks for confirmation before pushing:

```sh
> pr-tree replant --apply 1234

  #1235 (STEM) → rebased onto main
  #1236 (LEAF) → rebased onto feature-b

Force-push 2 branch(es) to origin? [y/N] y
  pushed feature-b
  pushed feature-c
    re-requested review: @alice

Replant complete.
```

Flags:

- `--apply` — perform the rebases and force-push (omit for dry-run).
- `-y` / `--yes` — skip the `Force-push N branch(es)? [y/N]` confirmation prompt.
- `--re-request-reviews` — after a successful force-push, re-request review from
  each pushed PR's approvers whose approval was staled by the push. In dry-run
  mode it lists who would be re-requested. Re-request failures are non-fatal.

If a rebase hits a conflict, `replant --apply` pauses with the rebase in
progress and prints guidance for resolving and resuming. Re-running after
resolving skips branches that were already successfully replanted.

After a squash-merge, the redundant parent commits are identified **structurally** — `replant` rebases each child with `git rebase --onto <new-base> <fork-point> <child>`, where the fork point is `git merge-base` of the parent's recorded head (GitHub's `headRefOid`) and the child's head. This drops exactly the merged commits regardless of squashing, where patch-id detection (`git cherry`) would not.

`go` moves between branches in a stack without having to remember branch names.
It must be run from an open PR's branch and aborts if the working tree is dirty.

```sh
> pr-tree go up      # check out the parent PR's branch
> pr-tree go down    # descend to a child (prompts if there is more than one)
> pr-tree go root    # jump to the nearest unmerged root of the tree
> pr-tree go leaf    # jump to the tip (prompts if the tree has multiple leaves)
```

Flags:

- `-n` / `--dry-run` — print the target branch (or the candidate list, when a choice would be required) instead of checking out.

> **Note:** `list`, `replant`, `annotate`, and `go` are implemented. See [ROADMAP.md](ROADMAP.md).

### Color

By default, `list` colors its output when writing to a terminal — titles are bold, and PR numbers, reviewers, status, and the review marker are colored to make dense forests easier to scan. Color is disabled automatically when output is piped or redirected. To force it off, pass `--no-color` or set the `NO_COLOR` environment variable.

A trailing green `✓` marks a PR that has received the reviews required to merge (GitHub's review decision is *approved*), and the line itself is rendered in bold so approved PRs stand out when scanning a stack. The `✓` is shown without color, and the bold is dropped, when color is disabled.

## Development

- [DEVELOPMENT.md](DEVELOPMENT.md) — repository structure, build/test/run commands, and architecture.
- [AGENTS.md](AGENTS.md) — conventions for AI coding agents.
- [ROADMAP.md](ROADMAP.md) — build status across `list` → `annotate` → `replant`.

