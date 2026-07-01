package main

import (
	"context"
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
	"github.com/alexkalderimis/pr-tree/internal/replant"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func newReplantCmd() *cobra.Command {
	var apply, yes, reRequest, noColor bool
	var parent, keep int
	cmd := &cobra.Command{
		Use:   "replant [#PR]",
		Short: "Rebase a PR's descendants (dry-run unless --apply)",
		Long: "Plan or perform the rebase of every descendant of a PR after the " +
			"PR merged (squash) or changed. The target defaults to the PR for the " +
			"current branch. Without --apply it only prints the plan; with --apply " +
			"it rebases each descendant and force-pushes them.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			color := colorEnabled(noColor, os.Getenv("NO_COLOR"), term.IsTerminal(int(os.Stdout.Fd())))
			if apply {
				return runApply(cmd.Context(), repoFlag, args, yes, reRequest, parent, keep, color, cmd.InOrStdin(), cmd.OutOrStdout())
			}
			return runReplant(cmd.Context(), repoFlag, args, reRequest, parent, keep, color, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Rebase and force-push (default: dry-run)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the pre-push confirmation prompt")
	cmd.Flags().BoolVar(&reRequest, "re-request-reviews", false, "Re-request review from approvers after force-push")
	cmd.Flags().IntVar(&parent, "parent", 0, "Override the target's upstream parent PR (bare number, no #) when its lineage can't be inferred — e.g. a squash-merged parent that caused GitHub to retarget the base")
	cmd.Flags().IntVar(&keep, "keep", 0, "Keep only the newest N commits of the target, dropping the rest as already-merged (overrides automatic fork detection)")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	return cmd
}

// mergedParentSubjects fetches the subject set used to recognise an
// already-merged parent's commits on a drifted target branch. It fetches the
// commits of the target step's parent PR and combines them with the parent's
// title. Returns nil (best-effort) when there is no target step or the fetch
// fails — fork detection then falls back to ancestry or --keep.
func mergedParentSubjects(ctx context.Context, client *github.Client, repo config.Repo, plan []replant.Step, byNum map[int]tree.PullRequest) map[string]bool {
	for _, s := range plan {
		if !s.TargetSelf {
			continue
		}
		parent := byNum[s.ParentPR]
		subjects, err := client.FetchPRCommitSubjects(ctx, repo, s.ParentPR)
		if err != nil {
			return nil
		}
		return subjectSet(subjects, parent.Title)
	}
	return nil
}

// injectParentOverride forces the target PR's upstream parent to be `parent` by
// prepending a synthetic `upstream: #parent` link to its body, ahead of any
// existing (and possibly wrong) upstream reference. The forest builder reads the
// first upstream link, so the override wins. This re-establishes lineage the
// tool otherwise can't infer — typically a squash-merged parent whose merge
// made GitHub retarget the target's base to the default branch.
func injectParentOverride(prs []tree.PullRequest, target, parent int) {
	for i := range prs {
		if prs[i].Number == target {
			prs[i].Body = fmt.Sprintf("upstream: #%d\n\n", parent) + prs[i].Body
			return
		}
	}
}

func runReplant(ctx context.Context, repoFlag string, args []string, reRequest bool, parent, keep int, color bool, out io.Writer) error {
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

	g := git.New("")
	target, err := resolveTarget(g, args, prs)
	if err != nil {
		return err
	}
	// An explicit --parent override must be applied before resolving missing
	// parents, so the injected upstream link is the one fetched and linked.
	if parent != 0 {
		injectParentOverride(prs, target, parent)
	}
	// Pull in merged/closed parents referenced by links so a just-merged target
	// (and its recorded head OID) is present in the tree.
	prs = append(prs, fetchMissingParents(ctx, client, repo, prs)...)

	byNum := make(map[int]tree.PullRequest, len(prs))
	for _, pr := range prs {
		byNum[pr.Number] = pr
	}

	forest := tree.BuildForest(prs, defaultBranch)
	plan, err := replant.Plan(forest, target, defaultBranch)
	if err != nil {
		return err
	}

	if len(plan) == 0 {
		fmt.Fprintln(out, "  (nothing to replant: no parent to move onto and no descendants)")
		return nil
	}

	mergedSubjects := mergedParentSubjects(ctx, client, repo, plan, byNum)
	footer := "\n(dry-run: no branches were rebased or pushed — pass --apply to execute)"
	view := buildReplantView(g, byNum, forest, defaultBranch, target, keep, plan, mergedSubjects, footer)
	fmt.Fprint(out, render.ReplantPlan(view, render.Options{Color: color}))

	if reRequest {
		for _, s := range plan {
			pr := byNum[s.PR]
			if len(pr.Approvers) == 0 {
				continue
			}
			logins := make([]string, 0, len(pr.Approvers))
			for _, a := range pr.Approvers {
				logins = append(logins, "@"+a.Login)
			}
			fmt.Fprintf(out, "Will re-request review: #%d %s\n", s.PR, strings.Join(logins, ", "))
		}
	}
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

// resolveDropped lists the commits a rebase sheds: those reachable from the
// fork point but not from baseRef (the same origin-qualified base the rebase
// lands on). Best-effort — returns nil if the range can't be listed.
func resolveDropped(g *git.Git, baseRef, fork string) []git.Commit {
	dropped, err := g.RevList(baseRef + ".." + fork)
	if err != nil {
		return nil
	}
	return dropped
}
