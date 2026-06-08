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
  └ #1236 (LEAF-2, reviewer: @bar, DRAFT)
#1234 (ANOTHER ROOT, reviewer: @abc, OPEN)
```

List PRs that are in your review queue, including the context of other PRs in the tree:

```sh
> pr-tree list --to-review

#1234 (A, reviewer: @foo, OPEN)
└ #1235 (B, OPEN) <== Review pending
  └ #1236 (C, reviewer: @bar, OPEN)
    └ #1237 (D, reviewer: @bar, DRAFT)
#1234 (Another one, OPEN) <== Review pending
```

Other commands:

- `replant` - rebase all descendants (infers the current PR from the current branch, accepts `[#PR_ID]` argument)
- `annotate` - update the PR descriptions of all PRs in the tree with a `links:` section including `upstream` and `downstream` links, so that the stack is navigable within GitHub



 




