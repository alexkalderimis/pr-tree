package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/alexkalderimis/pr-tree/internal/config"
	"github.com/alexkalderimis/pr-tree/internal/git"
	"github.com/alexkalderimis/pr-tree/internal/github"
	"github.com/alexkalderimis/pr-tree/internal/render"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func newListCmd() *cobra.Command {
	var mine, toReview, noColor, root, approved, active bool
	var parent, treeArg string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List PRs as trees",
		RunE: func(cmd *cobra.Command, args []string) error {
			color := colorEnabled(noColor, os.Getenv("NO_COLOR"), term.IsTerminal(int(os.Stdout.Fd())))
			sel, err := chooseSelector(root, cmd.Flags().Changed("parent"), cmd.Flags().Changed("tree"))
			if err != nil {
				return err
			}
			// The raw PRNO string for the active selector ("" means current branch).
			var prnoArg string
			switch sel {
			case selParent:
				prnoArg = parent
			case selTree:
				prnoArg = treeArg
			}
			opts := listOptions{
				mine: mine, toReview: toReview, approved: approved, active: active,
				sel: sel, prnoArg: prnoArg, color: color,
			}
			return runList(cmd.Context(), repoFlag, opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&mine, "mine", false, "Only show trees containing PRs you authored")
	cmd.Flags().BoolVar(&toReview, "to-review", false, "Only show trees containing PRs awaiting your review")
	cmd.Flags().BoolVar(&root, "root", false, "Only root nodes (PRs with no unmerged parent), shown flat")
	cmd.Flags().StringVar(&parent, "parent", "", "Tree of nodes descending from PR (empty = current branch's PR)")
	cmd.Flags().StringVar(&treeArg, "tree", "", "Whole tree containing PR, ancestors and descendants (empty = current branch's PR)")
	cmd.Flags().BoolVar(&approved, "approved", false, "Only PRs that have been approved")
	cmd.Flags().BoolVar(&active, "active", false, "Only PRs that are not drafts")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	return cmd
}

// listOptions bundles the resolved list flags for runList.
type listOptions struct {
	mine, toReview, approved, active bool
	sel                              selector
	prnoArg                          string // raw PRNO for selParent/selTree; "" = current branch
	color                            bool
}

func runList(ctx context.Context, repoFlag string, o listOptions, out io.Writer) error {
	repo, err := config.Resolve(repoFlag)
	if err != nil {
		return err
	}
	token, err := github.Token()
	if err != nil {
		return err
	}
	client := github.New(token)

	viewer, err := client.Viewer(ctx)
	if err != nil {
		return fmt.Errorf("looking up authenticated user: %w", err)
	}

	prs, defaultBranch, err := client.FetchOpenPRs(ctx, repo)
	if err != nil {
		return fmt.Errorf("fetching PRs for %s: %w", repo, err)
	}

	// Resolve merged/closed parents referenced by upstream links but not in the
	// open set, so they can anchor trees (the only way to place merged nodes).
	prs = append(prs, fetchMissingParents(ctx, client, repo, prs)...)

	forest := tree.BuildForest(prs, defaultBranch)

	// Stage 1: selector.
	working := forest
	switch o.sel {
	case selRoot:
		working = tree.LiveRoots(forest)
	case selParent, selTree:
		prno, err := resolvePRNo(o.prnoArg, prs)
		if err != nil {
			return err
		}
		if o.sel == selParent {
			working = tree.Subtree(forest, prno)
		} else {
			working = tree.WholeTree(forest, prno)
		}
	}

	// Stage 2: narrowing.
	filter := tree.Filter{
		Mine: o.mine, ToReview: o.toReview,
		Approved: o.approved, Active: o.active, Viewer: viewer,
	}
	if o.approved || o.active {
		working = tree.PruneNodes(working, filter.Keeps)
	}
	selected := tree.SelectTrees(working, filter)
	pending := tree.ReviewPending(working, filter)

	text := render.Render(selected, render.Options{ReviewPending: pending, Color: o.color})
	_, err = out.Write([]byte(text))
	return err
}

// resolvePRNo turns a selector's raw PRNO argument into a PR number. An empty
// argument means the PR of the current branch.
func resolvePRNo(arg string, prs []tree.PullRequest) (int, error) {
	if strings.TrimSpace(arg) != "" {
		return parsePRNumber(arg)
	}
	branch, err := git.New("").CurrentBranch()
	if err != nil {
		return 0, fmt.Errorf("inferring current PR: %w (pass an explicit PR number)", err)
	}
	if n, ok := prForBranch(branch, prs); ok {
		return n, nil
	}
	return 0, fmt.Errorf("no open PR has head branch %q — pass an explicit PR number", branch)
}

type selector int

const (
	selNone selector = iota
	selRoot
	selParent
	selTree
)

// chooseSelector resolves the active Stage-1 selector. --root, --parent, and
// --tree are mutually exclusive.
func chooseSelector(root, parentSet, treeSet bool) (selector, error) {
	n := 0
	if root {
		n++
	}
	if parentSet {
		n++
	}
	if treeSet {
		n++
	}
	if n > 1 {
		return selNone, errors.New("--root, --parent, and --tree are mutually exclusive")
	}
	switch {
	case root:
		return selRoot, nil
	case parentSet:
		return selParent, nil
	case treeSet:
		return selTree, nil
	default:
		return selNone, nil
	}
}

// parsePRNumber parses a PR number, tolerating a leading '#'.
func parsePRNumber(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	if err != nil {
		return 0, fmt.Errorf("invalid PR number %q", s)
	}
	return n, nil
}

// prForBranch returns the number of the first live PR whose head branch matches.
func prForBranch(branch string, prs []tree.PullRequest) (int, bool) {
	for _, pr := range prs {
		if pr.State.IsLive() && pr.HeadRef == branch {
			return pr.Number, true
		}
	}
	return 0, false
}

// fetchMissingParents looks up PRs referenced via `upstream:` links that are
// not already present (typically merged parents). Errors are non-fatal.
func fetchMissingParents(ctx context.Context, client *github.Client, repo config.Repo, prs []tree.PullRequest) []tree.PullRequest {
	have := make(map[int]bool, len(prs))
	for _, pr := range prs {
		have[pr.Number] = true
	}
	var missing []int
	seen := make(map[int]bool)
	for _, pr := range prs {
		if up := tree.ParseUpstream(pr.Body); up != 0 && !have[up] && !seen[up] {
			seen[up] = true
			missing = append(missing, up)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	extra, err := client.FetchPRsByNumber(ctx, repo, missing)
	if err != nil {
		return nil // best-effort; un-resolvable parents just become roots
	}
	return extra
}
