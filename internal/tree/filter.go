package tree

// Filter describes which PRs a list invocation cares about. With neither Mine
// nor ToReview set, every tree matches.
type Filter struct {
	Mine     bool
	ToReview bool
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
