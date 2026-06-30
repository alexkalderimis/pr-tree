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
	TargetSelf   bool // true for the target's own step: its parent sits above the
	// target and is NOT rebased in this run, so the command layer resolves this
	// step's new base from the parent's origin head rather than a local branch.
}

// Plan walks the descendants of the target PR top-down and produces the rebase
// steps. When the target has a parent, the target itself is rebased first (its
// parent merged → onto the default branch, or moved → onto the parent's head),
// marked TargetSelf; a root target only moves its descendants. Every parent
// appears before its children, so the command layer can rebase in order and
// feed each child its parent's freshly rebased head. It returns an error if the
// target PR is not in the forest.
func Plan(forest []*tree.Node, target int, defaultBranch string) ([]Step, error) {
	node, parent := findWithParent(forest, target, nil)
	if node == nil {
		return nil, fmt.Errorf("PR #%d is not in the tree", target)
	}

	var plan []Step
	parentOf := map[int]*tree.Node{}

	// If the target has a parent, the target itself may need re-homing: its
	// parent merged (→ default branch) or moved (→ parent head). Emit it first,
	// before its descendants. Whether the step is a real move or a no-op is
	// decided by the command layer, which has git access.
	if parent != nil {
		merged := parent.PR.State == tree.StateMerged
		newBase := parent.PR.HeadRef
		if merged {
			newBase = defaultBranch
		}
		plan = append(plan, Step{
			PR:           node.PR.Number,
			HeadRef:      node.PR.HeadRef,
			NewBaseRef:   newBase,
			ParentPR:     parent.PR.Number,
			ParentMerged: merged,
			TargetSelf:   true,
		})
	}

	// Breadth-first from the target guarantees parents precede children.
	queue := append([]*tree.Node(nil), node.Children...)
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

// findWithParent returns the node for num and its parent node (nil if num is a
// root or not present), searching the whole forest. parent threads the current
// parent through the recursion.
func findWithParent(forest []*tree.Node, num int, parent *tree.Node) (node, par *tree.Node) {
	for _, n := range forest {
		if n.PR.Number == num {
			return n, parent
		}
		if got, gotPar := findWithParent(n.Children, num, n); got != nil {
			return got, gotPar
		}
	}
	return nil, nil
}
