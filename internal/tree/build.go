package tree

import "sort"

// BuildForest reconstructs the PR forest. A PR's parent is the PR whose head
// branch equals this PR's base branch (branch topology). When branch topology
// finds no parent, an `upstream: #N` link in the body is used as a fallback,
// provided #N is present in the input. PRs with no resolved parent are roots.
// Roots and children are sorted by PR number for stable output.
func BuildForest(prs []PullRequest) []*Node {
	nodes := make(map[int]*Node, len(prs))
	byHead := make(map[string]int, len(prs)) // live head branch -> PR number
	for i := range prs {
		pr := prs[i]
		nodes[pr.Number] = &Node{PR: pr}
		if pr.State.IsLive() && pr.HeadRef != "" {
			byHead[pr.HeadRef] = pr.Number
		}
	}

	parentOf := make(map[int]int, len(prs)) // child number -> parent number
	for _, n := range nodes {
		pr := n.PR
		parent := 0
		if p, ok := byHead[pr.BaseRef]; ok && p != pr.Number {
			parent = p
		} else if up := ParseUpstream(pr.Body); up != 0 && up != pr.Number {
			if _, ok := nodes[up]; ok {
				parent = up
			}
		}
		if parent != 0 && !createsCycle(parentOf, pr.Number, parent) {
			parentOf[pr.Number] = parent
		}
	}

	var roots []*Node
	for num, n := range nodes {
		if parent, ok := parentOf[num]; ok {
			p := nodes[parent]
			p.Children = append(p.Children, n)
		} else {
			roots = append(roots, n)
		}
	}

	sortNodes(roots)
	return roots
}

// createsCycle reports whether making child a child of parent would create a
// cycle, by walking parent's existing ancestry.
func createsCycle(parentOf map[int]int, child, parent int) bool {
	for at := parent; at != 0; {
		if at == child {
			return true
		}
		at = parentOf[at]
	}
	return false
}

// sortNodes sorts a slice of nodes by PR number and recurses into children.
func sortNodes(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].PR.Number < nodes[j].PR.Number
	})
	for _, n := range nodes {
		sortNodes(n.Children)
	}
}
