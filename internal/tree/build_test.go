package tree

import "testing"

// numbers returns the PR numbers of the given nodes, in order.
func numbers(nodes []*Node) []int {
	out := make([]int, len(nodes))
	for i, n := range nodes {
		out[i] = n.PR.Number
	}
	return out
}

func eq(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestBuildForest_BranchTopology(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, State: StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: StateOpen, BaseRef: "b", HeadRef: "c"},
		{Number: 4, State: StateOpen, BaseRef: "main", HeadRef: "d"},
	}
	forest := BuildForest(prs)

	eq(t, numbers(forest), []int{1, 4}) // roots base on main, sorted by number
	eq(t, numbers(forest[0].Children), []int{2})
	eq(t, numbers(forest[0].Children[0].Children), []int{3})
}
