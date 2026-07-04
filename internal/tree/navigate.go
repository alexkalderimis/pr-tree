package tree

// Find returns the node for num anywhere in the forest, or nil.
func Find(roots []*Node, num int) *Node {
	for _, n := range roots {
		if n.PR.Number == num {
			return n
		}
		if found := Find(n.Children, num); found != nil {
			return found
		}
	}
	return nil
}

// Parent returns the parent node of num, or nil if num is a root or absent.
func Parent(roots []*Node, num int) *Node {
	var walk func(nodes []*Node, parent *Node) *Node
	walk = func(nodes []*Node, parent *Node) *Node {
		for _, n := range nodes {
			if n.PR.Number == num {
				return parent
			}
			if p := walk(n.Children, n); p != nil {
				return p
			}
		}
		return nil
	}
	return walk(roots, nil)
}

// Root returns the topmost ancestor of num — itself if it is a root. Because the
// forest is built from live PRs only, merged ancestors are absent, so this is
// the nearest unmerged root of num's tree. Returns nil if num is absent.
func Root(roots []*Node, num int) *Node {
	n := Find(roots, num)
	if n == nil {
		return nil
	}
	for {
		p := Parent(roots, n.PR.Number)
		if p == nil {
			return n
		}
		n = p
	}
}

// Leaves returns the leaf descendants of the subtree rooted at node (nodes with
// no children), in stable PR-number order. A node with no children is its own
// only leaf. Returns nil when node is nil.
func Leaves(node *Node) []*Node {
	if node == nil {
		return nil
	}
	if len(node.Children) == 0 {
		return []*Node{node}
	}
	var out []*Node
	for _, c := range node.Children {
		out = append(out, Leaves(c)...)
	}
	return out
}
