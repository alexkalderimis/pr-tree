package tree

import "testing"

// linear builds #1 -> #2 -> #3 (each PR's base is the previous head).
func linearForest() []*Node {
	prs := []PullRequest{
		{Number: 1, State: StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: StateOpen, BaseRef: "b", HeadRef: "c"},
	}
	return BuildForest(prs, "main")
}

// branching builds #1 -> #2 -> #3 and #1 -> #4 (two leaves under #1).
func branchingForest() []*Node {
	prs := []PullRequest{
		{Number: 1, State: StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: StateOpen, BaseRef: "b", HeadRef: "c"},
		{Number: 4, State: StateOpen, BaseRef: "a", HeadRef: "d"},
	}
	return BuildForest(prs, "main")
}

func TestFind(t *testing.T) {
	f := linearForest()
	if n := Find(f, 2); n == nil || n.PR.Number != 2 {
		t.Fatalf("Find(2) = %v, want node #2", n)
	}
	if n := Find(f, 99); n != nil {
		t.Fatalf("Find(99) = %v, want nil", n)
	}
}

func TestParent(t *testing.T) {
	f := linearForest()
	if p := Parent(f, 2); p == nil || p.PR.Number != 1 {
		t.Fatalf("Parent(2) = %v, want #1", p)
	}
	if p := Parent(f, 3); p == nil || p.PR.Number != 2 {
		t.Fatalf("Parent(3) = %v, want #2", p)
	}
	if p := Parent(f, 1); p != nil {
		t.Fatalf("Parent(1) = %v, want nil (root)", p)
	}
}

// When the structural root's branch is absent (merged parent deleted), the
// topmost node in the live-only forest is the nearest unmerged root.
func TestRoot_TopmostUnmerged(t *testing.T) {
	// #2's base "a" has no live PR, so #2 is a root; #3 hangs off #2.
	prs := []PullRequest{
		{Number: 2, State: StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: StateOpen, BaseRef: "b", HeadRef: "c"},
	}
	f := BuildForest(prs, "main")
	if r := Root(f, 3); r == nil || r.PR.Number != 2 {
		t.Fatalf("Root(3) = %v, want #2", r)
	}
	if r := Root(f, 2); r == nil || r.PR.Number != 2 {
		t.Fatalf("Root(2) = %v, want itself #2", r)
	}
}

func TestLeaves(t *testing.T) {
	f := branchingForest()
	one := Find(f, 1)
	eq(t, numbers(Leaves(one)), []int{3, 4})

	two := Find(f, 2)
	eq(t, numbers(Leaves(two)), []int{3})

	four := Find(f, 4)
	eq(t, numbers(Leaves(four)), []int{4}) // a leaf is its own only leaf
}
