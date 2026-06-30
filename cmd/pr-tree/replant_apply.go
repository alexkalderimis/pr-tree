package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/alexkalderimis/pr-tree/internal/config"
	"github.com/alexkalderimis/pr-tree/internal/git"
	"github.com/alexkalderimis/pr-tree/internal/github"
	"github.com/alexkalderimis/pr-tree/internal/replant"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func runApply(ctx context.Context, repoFlag string, args []string, yes, reRequest bool, parent int, in io.Reader, out io.Writer) error {
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
	if inProg, err := g.RebaseInProgress(); err != nil {
		return err
	} else if inProg {
		return errors.New("a rebase is already in progress — finish it with 'git rebase --continue' or 'git rebase --abort', then re-run")
	}
	if clean, err := g.WorktreeClean(); err != nil {
		return err
	} else if !clean {
		return errors.New("working tree has uncommitted changes — commit or stash them first")
	}

	target, err := resolveTarget(g, args, prs)
	if err != nil {
		return err
	}
	// An explicit --parent override must be applied before resolving missing
	// parents, so the injected upstream link is the one fetched and linked.
	if parent != 0 {
		injectParentOverride(prs, target, parent)
	}
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
		fmt.Fprintln(out, "Nothing to replant: the target has no parent to move onto and no descendants.")
		return nil
	}

	startBranch, err := g.CurrentBranch()
	if err != nil {
		return err
	}
	if err := g.Fetch("origin"); err != nil {
		return fmt.Errorf("fetching origin: %w", err)
	}

	// Phase A: rebase every descendant locally; nothing is pushed yet.
	var rebased []replant.Step
	for _, s := range plan {
		child := byNum[s.PR]
		parent := byNum[s.ParentPR]
		_ = g.FetchOID("origin", parent.HeadOID)
		_ = g.FetchOID("origin", child.HeadOID)

		newBaseOID, err := resolveNewBase(g, defaultBranch, s, parent.HeadOID)
		if err != nil {
			g.Checkout(startBranch)
			return err
		}
		done, err := targetOrDescendantPlaced(g, s, newBaseOID, parent.HeadOID, child.HeadOID)
		if err != nil {
			g.Checkout(startBranch)
			return err
		}
		if done {
			if s.TargetSelf {
				fmt.Fprintf(out, "  #%d (%s) ✓ already based on #%d — no change\n", s.PR, child.Title, s.ParentPR)
			} else {
				fmt.Fprintf(out, "  #%d (%s) ✓ already replanted, skipping\n", s.PR, child.Title)
			}
			continue
		}
		if err := g.PrepareBranch(s.HeadRef, child.HeadOID); err != nil {
			g.Checkout(startBranch)
			return err
		}
		fork, err := g.MergeBase(parent.HeadOID, child.HeadOID)
		if err != nil {
			g.Checkout(startBranch)
			return err
		}
		if err := g.Rebase(newBaseOID, fork, s.HeadRef); err != nil {
			if errors.Is(err, git.ErrRebaseConflict) {
				fmt.Fprintf(out, "\nConflict rebasing #%d (%s) on branch %q.\n", s.PR, child.Title, s.HeadRef)
				fmt.Fprintln(out, "Resolve it, run 'git rebase --continue', then re-run 'pr-tree replant --apply' to finish the rest.")
				fmt.Fprintln(out, "(nothing has been pushed)")
				return fmt.Errorf("rebase conflict on #%d", s.PR)
			}
			g.Checkout(startBranch)
			return err
		}
		fmt.Fprintf(out, "  #%d (%s) → rebased onto %s\n", s.PR, child.Title, s.NewBaseRef)
		rebased = append(rebased, s)
	}

	if len(rebased) == 0 {
		g.Checkout(startBranch)
		fmt.Fprintln(out, "All descendants already replanted; nothing to push.")
		return nil
	}

	// Phase B: confirm, then force-push everything that was rebased.
	if !yes {
		fmt.Fprintf(out, "\nForce-push %d branch(es) to origin? [y/N] ", len(rebased))
		if !confirmed(in) {
			g.Checkout(startBranch)
			fmt.Fprintln(out, "Aborted; no branches were pushed.")
			return nil
		}
	}
	for _, s := range rebased {
		if err := g.PushForceWithLease("origin", s.HeadRef); err != nil {
			g.Checkout(startBranch)
			return fmt.Errorf("pushing %s: %w", s.HeadRef, err)
		}
		fmt.Fprintf(out, "  pushed %s\n", s.HeadRef)
		if reRequest {
			pr := byNum[s.PR]
			if len(pr.Approvers) > 0 {
				ids := make([]string, 0, len(pr.Approvers))
				logins := make([]string, 0, len(pr.Approvers))
				for _, a := range pr.Approvers {
					ids = append(ids, a.ID)
					logins = append(logins, "@"+a.Login)
				}
				if err := client.RequestReviews(ctx, pr.NodeID, ids); err != nil {
					fmt.Fprintf(out, "    warning: could not re-request reviews on #%d: %v\n", s.PR, err)
				} else {
					fmt.Fprintf(out, "    re-requested review: %s\n", strings.Join(logins, ", "))
				}
			}
		}
	}
	g.Checkout(startBranch)
	fmt.Fprintln(out, "\nReplant complete.")
	return nil
}

// baseRef returns the git ref (or OID) whose commit a step rebases onto.
// A merged parent → the default branch on origin. The target's own step rebases
// onto its parent's origin head (the parent is above the target and was not
// rebased in this run). A descendant rebases onto its parent's local branch,
// which an earlier step in this run already rebased.
func baseRef(defaultBranch string, s replant.Step, parentHeadOID string) string {
	if s.ParentMerged {
		return "origin/" + defaultBranch
	}
	if s.TargetSelf {
		return parentHeadOID
	}
	return s.NewBaseRef
}

// resolveNewBase turns a Step's intended base into a concrete OID.
func resolveNewBase(g *git.Git, defaultBranch string, s replant.Step, parentHeadOID string) (string, error) {
	return g.RevParse(baseRef(defaultBranch, s, parentHeadOID))
}

// targetOrDescendantPlaced reports whether a step needs no work. For an open
// target step the predicate is "the parent's current head is already an
// ancestor of the target head" (nothing moved). For a merged parent, or any
// descendant, the standard AlreadyReplanted check applies.
func targetOrDescendantPlaced(g *git.Git, s replant.Step, newBaseOID, parentHeadOID, childHeadOID string) (bool, error) {
	if s.TargetSelf && !s.ParentMerged {
		return g.IsAncestor(parentHeadOID, childHeadOID)
	}
	return g.AlreadyReplanted(newBaseOID, parentHeadOID, s.HeadRef)
}

// confirmed reads a line and reports whether it is an affirmative y/yes.
func confirmed(in io.Reader) bool {
	line, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
