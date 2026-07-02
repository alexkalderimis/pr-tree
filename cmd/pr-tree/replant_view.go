package main

import (
	"errors"
	"strconv"

	"github.com/alexkalderimis/pr-tree/internal/git"
	"github.com/alexkalderimis/pr-tree/internal/render"
	"github.com/alexkalderimis/pr-tree/internal/replant"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// findNode returns the node for num anywhere in the forest, or nil.
func findNode(forest []*tree.Node, num int) *tree.Node {
	for _, n := range forest {
		if n.PR.Number == num {
			return n
		}
		if got := findNode(n.Children, num); got != nil {
			return got
		}
	}
	return nil
}

// buildReplantView assembles the render model for a replant plan. It shows
// Before/After when the target is re-homed onto the default branch (its
// TargetSelf step has a merged parent), else a single Stack. It computes the
// dropped commits for the target step via the shared fork logic; a
// forkUnknownError becomes a warning instead of a list.
func buildReplantView(g *git.Git, byNum map[int]tree.PullRequest, forest []*tree.Node, defaultBranch string, target, keep int, plan []replant.Step, mergedSubjects map[string]bool, footer string) render.ReplantPlanInput {
	tpr := byNum[target]
	in := render.ReplantPlanInput{
		Header: "Replant plan for #" + strconv.Itoa(target) + " (" + tpr.Title + ")",
		Target: findNode(forest, target),
		Footer: footer,
	}

	var ts *replant.Step
	for i := range plan {
		if plan[i].TargetSelf {
			ts = &plan[i]
			break
		}
	}
	if ts == nil {
		return in
	}

	parent := byNum[ts.ParentPR]
	if ts.ParentMerged {
		in.Reparented = true
		op := parent
		in.OldParent = &op
		in.NewBaseLabel = defaultBranch
	}

	child := byNum[ts.PR]
	_ = g.FetchOID("origin", parent.HeadOID)
	_ = g.FetchOID("origin", child.HeadOID)
	fb := baseRef(defaultBranch, *ts, parent.HeadOID)
	fork, err := resolveFork(g, fb, *ts, child, parent, keep, mergedSubjects)
	if err != nil {
		var unknown *forkUnknownError
		if errors.As(err, &unknown) {
			in.ForkWarning = err.Error()
		}
		return in
	}
	for _, c := range resolveDropped(g, fb, fork.OID) {
		in.Dropped = append(in.Dropped, render.Commit{OID: c.OID, Subject: c.Subject})
	}
	in.DropVia = ts.ParentPR
	return in
}
