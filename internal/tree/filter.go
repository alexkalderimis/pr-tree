package tree

// Filter describes which PRs a list invocation cares about. With neither Mine
// nor ToReview set, every tree matches.
type Filter struct {
	Mine     bool
	ToReview bool
	Approved bool
	Active   bool
	Viewer   string // the authenticated user's login
}

func contains(xs []string, target string) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}

// matches reports whether a single PR satisfies the filter.
func (f Filter) matches(pr PullRequest) bool {
	if !f.Mine && !f.ToReview {
		return true
	}
	if f.Mine && pr.Author == f.Viewer {
		return true
	}
	if f.ToReview && contains(pr.Reviewers, f.Viewer) {
		return true
	}
	return false
}

// anyMatch reports whether any node in the subtree rooted at n matches.
func anyMatch(n *Node, f Filter) bool {
	if f.matches(n.PR) {
		return true
	}
	for _, c := range n.Children {
		if anyMatch(c, f) {
			return true
		}
	}
	return false
}

// SelectTrees returns the roots of whole trees that contain at least one
// matching node. Trees are returned in their existing order.
func SelectTrees(forest []*Node, f Filter) []*Node {
	var out []*Node
	for _, root := range forest {
		if anyMatch(root, f) {
			out = append(out, root)
		}
	}
	return out
}

// ReviewPending returns the set of PR numbers awaiting the viewer's review,
// i.e. nodes that request the viewer as a reviewer when ToReview is set.
func ReviewPending(forest []*Node, f Filter) map[int]bool {
	out := make(map[int]bool)
	if !f.ToReview {
		return out
	}
	var walk func(n *Node)
	walk = func(n *Node) {
		if contains(n.PR.Reviewers, f.Viewer) {
			out[n.PR.Number] = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, root := range forest {
		walk(root)
	}
	return out
}

// Subtree returns a single-root forest containing the node for prNo and all its
// descendants, or nil if prNo is not present in the forest.
func Subtree(forest []*Node, prNo int) []*Node {
	if n := findNode(forest, prNo); n != nil {
		return []*Node{n}
	}
	return nil
}

// WholeTree returns a single-root forest for the tree containing prNo: the
// forest root reachable by walking up from prNo. Returns nil if prNo is absent.
func WholeTree(forest []*Node, prNo int) []*Node {
	if findNode(forest, prNo) == nil {
		return nil
	}
	for _, root := range forest {
		if findNode([]*Node{root}, prNo) != nil {
			return []*Node{root}
		}
	}
	return nil
}

// findNode returns the node for prNo anywhere in the forest, or nil.
func findNode(forest []*Node, prNo int) *Node {
	for _, n := range forest {
		if n.PR.Number == prNo {
			return n
		}
		if found := findNode(n.Children, prNo); found != nil {
			return found
		}
	}
	return nil
}

// Keeps reports whether a PR survives the node-level narrowing filters
// (--approved / --active). Unlike matches, it does not consider Mine/ToReview.
// Exported so cmd/pr-tree can pass it as a method value to PruneNodes.
func (f Filter) Keeps(pr PullRequest) bool {
	if f.Approved && pr.ReviewDecision != ReviewApproved {
		return false
	}
	if f.Active && pr.State == StateDraft {
		return false
	}
	return true
}

// PruneNodes returns a new forest keeping only nodes where keep(pr) is true.
// A dropped node's surviving descendants re-attach to their nearest surviving
// ancestor, or become roots when no ancestor survives. The input is not
// mutated.
func PruneNodes(forest []*Node, keep func(PullRequest) bool) []*Node {
	var roots []*Node
	var walk func(n *Node, parent *Node)
	walk = func(n *Node, parent *Node) {
		next := parent
		if keep(n.PR) {
			copied := &Node{PR: n.PR}
			if parent == nil {
				roots = append(roots, copied)
			} else {
				parent.Children = append(parent.Children, copied)
			}
			next = copied
		}
		for _, c := range n.Children {
			walk(c, next)
		}
	}
	for _, root := range forest {
		walk(root, nil)
	}
	return roots
}

// LiveRoots returns a flat list of "live roots": every node with no unmerged
// parent — i.e. a forest root, or a node whose parent is MERGED. Only
// StateMerged counts; a CLOSED parent does not make its child a live root.
// Returned nodes are shallow copies with children cleared, so the result
// renders flat and the input forest is not mutated.
func LiveRoots(forest []*Node) []*Node {
	var out []*Node
	var walk func(n *Node, parentMerged, isRoot bool)
	walk = func(n *Node, parentMerged, isRoot bool) {
		if isRoot || parentMerged {
			out = append(out, &Node{PR: n.PR})
		}
		for _, c := range n.Children {
			walk(c, n.PR.State == StateMerged, false)
		}
	}
	for _, root := range forest {
		walk(root, false, true)
	}
	return out
}
