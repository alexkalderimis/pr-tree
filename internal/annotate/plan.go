package annotate

import "github.com/alexkalderimis/pr-tree/internal/tree"

// Update is the planned annotation for a single PR.
type Update struct {
	PR       tree.PullRequest
	Parent   int   // immediate parent PR number, 0 if root
	Children []int // immediate child PR numbers, in forest order
	OldBody  string
	NewBody  string
	Changed  bool // NewBody != OldBody
}

// Plan walks the forest in pre-order and computes, for every node, the body it
// should have after upserting its links block. It is pure.
func Plan(forest []*tree.Node) []Update {
	var out []Update
	var walk func(parent int, n *tree.Node)
	walk = func(parent int, n *tree.Node) {
		children := make([]int, 0, len(n.Children))
		for _, c := range n.Children {
			children = append(children, c.PR.Number)
		}
		block := RenderBlock(parent, children)
		newBody := Upsert(n.PR.Body, block)
		out = append(out, Update{
			PR:       n.PR,
			Parent:   parent,
			Children: children,
			OldBody:  n.PR.Body,
			NewBody:  newBody,
			Changed:  newBody != n.PR.Body,
		})
		for _, c := range n.Children {
			walk(n.PR.Number, c)
		}
	}
	for _, root := range forest {
		walk(0, root)
	}
	return out
}
