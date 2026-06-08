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
	forest := BuildForest(prs, "main")

	eq(t, numbers(forest), []int{1, 4}) // roots base on main, sorted by number
	eq(t, numbers(forest[0].Children), []int{2})
	eq(t, numbers(forest[0].Children[0].Children), []int{3})
}

func TestBuildForest_MergedParentViaLink(t *testing.T) {
	prs := []PullRequest{
		// Parent merged: its branch "a" is gone, so #2 was retargeted to main.
		{Number: 1, State: StateMerged, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: StateOpen, BaseRef: "main", HeadRef: "b", Body: "upstream: #1"},
	}
	forest := BuildForest(prs, "main")

	eq(t, numbers(forest), []int{1})             // merged #1 is the root
	eq(t, numbers(forest[0].Children), []int{2}) // #2 attached via link
}

func TestBuildForest_BranchTopologyBeatsLink(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, State: StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 5, State: StateOpen, BaseRef: "main", HeadRef: "e"},
		// Base branch "a" matches #1 by topology; stale link points at #5.
		{Number: 2, State: StateOpen, BaseRef: "a", HeadRef: "b", Body: "upstream: #5"},
	}
	forest := BuildForest(prs, "main")

	eq(t, numbers(forest), []int{1, 5})
	eq(t, numbers(forest[0].Children), []int{2}) // topology wins: child of #1
	if len(forest[1].Children) != 0 {
		t.Fatalf("#5 should have no children, got %v", numbers(forest[1].Children))
	}
}

func TestBuildForest_CycleGuard(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, State: StateOpen, BaseRef: "b", HeadRef: "a"},
		{Number: 2, State: StateOpen, BaseRef: "a", HeadRef: "b"},
	}
	forest := BuildForest(prs, "main") // must not infinite-loop

	// With sorted iteration, #1 is processed first: byHead["b"]=#2 exists, so
	// #1 becomes a child of #2. Then #2's parent would be #1, but that creates
	// a cycle and is rejected, so #2 is the root.
	eq(t, numbers(forest), []int{2})
	eq(t, numbers(forest[0].Children), []int{1})
}

func TestBuildForest_DefaultBranchBaseIsRootDespiteLink(t *testing.T) {
	// #2 targets the default branch ("main") but carries a stale upstream link
	// to #1. Because its base is the default branch, it MUST remain a root —
	// the link must not re-parent it.
	prs := []PullRequest{
		{Number: 1, State: StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: StateOpen, BaseRef: "main", HeadRef: "b", Body: "upstream: #1"},
	}
	forest := BuildForest(prs, "main")

	eq(t, numbers(forest), []int{1, 2}) // both roots
	if len(forest[0].Children) != 0 {
		t.Fatalf("#1 should have no children, got %v", numbers(forest[0].Children))
	}
}

func TestBuildForest_UnannotatedMergedParentAbsent(t *testing.T) {
	// #1 merged, branch "a" deleted, and (crucially) NOT in the input set at all
	// because it was never annotated so we never knew to fetch it. #2 was
	// retargeted to main and has no upstream link. #2 must be a plain root.
	prs := []PullRequest{
		{Number: 2, State: StateOpen, BaseRef: "main", HeadRef: "b"},
	}
	forest := BuildForest(prs, "main")

	eq(t, numbers(forest), []int{2})
	if len(forest[0].Children) != 0 {
		t.Fatalf("#2 should be a childless root, got %v", numbers(forest[0].Children))
	}
}
