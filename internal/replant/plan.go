// Package replant holds the pure planning logic for the `replant` command:
// given a forest of pull requests and a target PR, it decides which descendants
// must be rebased, in what order, and onto what new base. It performs no git or
// network I/O — the command layer resolves the concrete commit ranges.
package replant

import (
	"fmt"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// Step describes one descendant rebase. The command layer turns a Step into a
// concrete `git rebase --onto <NewBaseRef> <fork-point> <HeadRef>`, where the
// fork point is the merge-base of the parent PR's pre-rebase head and HeadRef.
type Step struct {
	PR         int    // the PR being rebased
	HeadRef    string // the PR's head branch (the rebase target / child head)
	NewBaseRef string // branch this PR should sit on: the default branch when
	// the parent merged, else the parent's head branch
	ParentPR     int  // the parent PR whose pre-rebase head gives the fork point
	ParentMerged bool // whether the parent was merged (squash) rather than changed
}

// Plan walks the descendants of the target PR top-down and produces the rebase
// steps. The target itself is not rebased (it is the PR that merged or changed);
// only its descendants move. Every parent appears before its children, so the
// command layer can rebase in order and feed each child its parent's freshly
// rebased head. It returns an error if the target PR is not in the forest.
func Plan(forest []*tree.Node, target int, defaultBranch string) ([]Step, error) {
	node := find(forest, target)
	if node == nil {
		return nil, fmt.Errorf("PR #%d is not in the tree", target)
	}

	var plan []Step
	// Breadth-first from the target guarantees parents precede children.
	queue := append([]*tree.Node(nil), node.Children...)
	parentOf := map[int]*tree.Node{}
	for _, c := range node.Children {
		parentOf[c.PR.Number] = node
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		parent := parentOf[n.PR.Number]

		merged := parent.PR.State == tree.StateMerged
		newBase := parent.PR.HeadRef
		if merged {
			newBase = defaultBranch
		}
		plan = append(plan, Step{
			PR:           n.PR.Number,
			HeadRef:      n.PR.HeadRef,
			NewBaseRef:   newBase,
			ParentPR:     parent.PR.Number,
			ParentMerged: merged,
		})

		for _, c := range n.Children {
			parentOf[c.PR.Number] = n
			queue = append(queue, c)
		}
	}
	return plan, nil
}

// find returns the node for the given PR number anywhere in the forest, or nil.
func find(forest []*tree.Node, num int) *tree.Node {
	for _, n := range forest {
		if n.PR.Number == num {
			return n
		}
		if got := find(n.Children, num); got != nil {
			return got
		}
	}
	return nil
}
