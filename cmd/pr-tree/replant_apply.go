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

func runApply(ctx context.Context, repoFlag string, args []string, yes, reRequest bool, in io.Reader, out io.Writer) error {
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
	prs = append(prs, fetchMissingParents(ctx, client, repo, prs)...)

	byNum := make(map[int]tree.PullRequest, len(prs))
	for _, pr := range prs {
		byNum[pr.Number] = pr
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
	forest := tree.BuildForest(prs, defaultBranch)
	plan, err := replant.Plan(forest, target, defaultBranch)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		fmt.Fprintln(out, "Nothing to replant: the target has no descendants.")
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

		newBaseOID, err := resolveNewBase(g, defaultBranch, s)
		if err != nil {
			g.Checkout(startBranch)
			return err
		}
		if done, err := g.AlreadyReplanted(newBaseOID, parent.HeadOID, s.HeadRef); err != nil {
			g.Checkout(startBranch)
			return err
		} else if done {
			fmt.Fprintf(out, "  #%d (%s) ✓ already replanted, skipping\n", s.PR, child.Title)
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
		// Task 6 inserts the re-request-reviews call here.
	}
	g.Checkout(startBranch)
	fmt.Fprintln(out, "\nReplant complete.")
	return nil
}

// resolveNewBase turns a Step's NewBaseRef into a concrete OID. A merged parent
// means the default branch on origin; otherwise it is the parent's local head,
// which was rebased earlier in this run.
func resolveNewBase(g *git.Git, defaultBranch string, s replant.Step) (string, error) {
	if s.ParentMerged {
		return g.RevParse("origin/" + defaultBranch)
	}
	return g.RevParse(s.NewBaseRef)
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
