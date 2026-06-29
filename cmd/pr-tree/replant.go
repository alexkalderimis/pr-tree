package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexkalderimis/pr-tree/internal/config"
	"github.com/alexkalderimis/pr-tree/internal/git"
	"github.com/alexkalderimis/pr-tree/internal/github"
	"github.com/alexkalderimis/pr-tree/internal/replant"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func newReplantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replant [#PR]",
		Short: "Show how a PR's descendants would be rebased (dry-run)",
		Long: "Plan the rebase of every descendant of a PR after the PR merged " +
			"(squash) or changed. The target defaults to the PR for the current " +
			"branch. This is currently dry-run only: it prints the plan and the " +
			"commits each descendant would drop and keep, but performs no rebase " +
			"or force-push.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReplant(cmd.Context(), repoFlag, args, cmd.OutOrStdout())
		},
	}
	return cmd
}

func runReplant(ctx context.Context, repoFlag string, args []string, out io.Writer) error {
	repo, err := config.Resolve(repoFlag)
	if err != nil {
		return err
	}
	token, err := github.Token()
	if err != nil {
		return err
	}
	client := github.New(token)

	prs, defaultBranch, err := client.FetchOpenPRs(ctx, repo)
	if err != nil {
		return fmt.Errorf("fetching PRs for %s: %w", repo, err)
	}
	// Pull in merged/closed parents referenced by links so a just-merged target
	// (and its recorded head OID) is present in the tree.
	prs = append(prs, fetchMissingParents(ctx, client, repo, prs)...)

	g := git.New("")
	byNum := make(map[int]tree.PullRequest, len(prs))
	for _, pr := range prs {
		byNum[pr.Number] = pr
	}

	target, err := resolveTarget(g, args, prs)
	if err != nil {
		return err
	}

	forest := tree.BuildForest(prs, defaultBranch)
	plan, err := replant.Plan(forest, target, defaultBranch)
	if err != nil {
		return err
	}

	tpr := byNum[target]
	reason := "restacking descendants onto its updated head"
	if tpr.State == tree.StateMerged {
		reason = fmt.Sprintf("merged — moving children onto %s", defaultBranch)
	}
	fmt.Fprintf(out, "Replant plan for #%d (%s) — %s:\n\n", target, tpr.Title, reason)

	if len(plan) == 0 {
		fmt.Fprintln(out, "  (no descendants to replant)")
		return nil
	}

	for _, s := range plan {
		printStep(out, g, byNum, defaultBranch, s)
	}

	fmt.Fprintf(out, "\n(dry-run: no branches were rebased or pushed — execution is not yet implemented)\n")
	return nil
}

// resolveTarget picks the PR to replant from: an explicit `#PR`/`PR` argument,
// or the PR whose head branch is currently checked out.
func resolveTarget(g *git.Git, args []string, prs []tree.PullRequest) (int, error) {
	if len(args) == 1 {
		n, err := strconv.Atoi(strings.TrimPrefix(args[0], "#"))
		if err != nil {
			return 0, fmt.Errorf("invalid PR number %q", args[0])
		}
		return n, nil
	}
	branch, err := g.CurrentBranch()
	if err != nil {
		return 0, fmt.Errorf("inferring current PR: %w (pass an explicit #PR)", err)
	}
	for _, pr := range prs {
		if pr.State.IsLive() && pr.HeadRef == branch {
			return pr.Number, nil
		}
	}
	return 0, fmt.Errorf("no open PR has head branch %q — pass an explicit #PR", branch)
}

// printStep renders one rebase, resolving the fork point and the drop/keep
// commit ranges via git. Resolution failures degrade to a note rather than
// aborting the whole dry-run.
func printStep(out io.Writer, g *git.Git, byNum map[int]tree.PullRequest, defaultBranch string, s replant.Step) {
	child := byNum[s.PR]
	parent := byNum[s.ParentPR]

	was := ""
	if child.BaseRef != s.NewBaseRef {
		was = fmt.Sprintf(" (was %s)", child.BaseRef)
	}
	fmt.Fprintf(out, "  #%d (%s) → rebase onto %s%s\n", s.PR, child.Title, s.NewBaseRef, was)

	// Localize both endpoints; merged parents may have deleted branches.
	_ = g.FetchOID("origin", parent.HeadOID)
	_ = g.FetchOID("origin", child.HeadOID)

	fork, err := g.MergeBase(parent.HeadOID, child.HeadOID)
	if err != nil {
		fmt.Fprintf(out, "      (could not compute fork point: %v)\n", err)
		return
	}

	if dropped := resolveDropped(g, defaultBranch, s.NewBaseRef, fork); len(dropped) > 0 {
		via := fmt.Sprintf("parent #%d", s.ParentPR)
		if s.ParentMerged {
			via = fmt.Sprintf("merged via #%d", s.ParentPR)
		}
		fmt.Fprintf(out, "      drop %s  (%s)\n", summarize(dropped), via)
	}

	kept, err := g.RevList(fork + ".." + child.HeadOID)
	if err != nil {
		fmt.Fprintf(out, "      (could not list kept commits: %v)\n", err)
		return
	}
	fmt.Fprintf(out, "      keep %s\n", summarize(kept))
}

// resolveDropped lists the parent commits the rebase sheds: the range from the
// new base to the fork point. Best-effort — returns nil if the new base can't
// be resolved locally.
func resolveDropped(g *git.Git, defaultBranch, newBaseRef, fork string) []git.Commit {
	base, err := g.RevParse(newBaseRef)
	if err != nil {
		if base, err = g.RevParse("origin/" + newBaseRef); err != nil {
			return nil
		}
	}
	dropped, err := g.RevList(base + ".." + fork)
	if err != nil {
		return nil
	}
	return dropped
}

// summarize renders a commit count and a short preview of the range.
func summarize(commits []git.Commit) string {
	n := len(commits)
	noun := "commits"
	if n == 1 {
		noun = "commit"
	}
	newest := short(commits[0].OID)
	oldest := short(commits[n-1].OID)
	if n == 1 {
		return fmt.Sprintf("%d %s  %s %s", n, noun, newest, commits[0].Subject)
	}
	return fmt.Sprintf("%d %s  %s..%s", n, noun, oldest, newest)
}

func short(oid string) string {
	if len(oid) > 7 {
		return oid[:7]
	}
	return oid
}
