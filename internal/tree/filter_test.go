package tree

import (
	"sort"
	"testing"
)

func sampleForest() []*Node {
	prs := []PullRequest{
		{Number: 1, State: StateOpen, Author: "alice", BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: StateOpen, Author: "bob", Reviewers: []string{"alice"}, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: StateOpen, Author: "carol", BaseRef: "main", HeadRef: "c"},
	}
	return BuildForest(prs, "main")
}

func TestSelectTrees_Mine(t *testing.T) {
	got := SelectTrees(sampleForest(), Filter{Mine: true, Viewer: "alice"})
	eq(t, numbers(got), []int{1}) // tree rooted at #1 contains alice's PR
}

func TestSelectTrees_ToReview(t *testing.T) {
	got := SelectTrees(sampleForest(), Filter{ToReview: true, Viewer: "alice"})
	eq(t, numbers(got), []int{1}) // #2 (in tree 1) requests alice as reviewer
}

func TestSelectTrees_NoFilterShowsAll(t *testing.T) {
	got := SelectTrees(sampleForest(), Filter{})
	eq(t, numbers(got), []int{1, 3})
}

func TestReviewPending(t *testing.T) {
	pending := ReviewPending(sampleForest(), Filter{ToReview: true, Viewer: "alice"})
	var keys []int
	for k := range pending {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	eq(t, keys, []int{2})
}

func TestSubtree(t *testing.T) {
	// #1 -> #2 -> #3 ; #1 -> #4
	prs := []PullRequest{
		{Number: 1, State: StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: StateOpen, BaseRef: "b", HeadRef: "c"},
		{Number: 4, State: StateOpen, BaseRef: "a", HeadRef: "d"},
	}
	forest := BuildForest(prs, "main")

	got := Subtree(forest, 2)
	eq(t, numbers(got), []int{2}) // #2 is the single root of the returned forest
	if len(got) != 1 || numbers(got[0].Children) == nil {
		t.Fatalf("expected #2 to retain child #3")
	}
	eq(t, numbers(got[0].Children), []int{3})

	if Subtree(forest, 99) != nil {
		t.Fatalf("absent PR should yield nil")
	}
}

func TestWholeTree(t *testing.T) {
	// #1 -> #2 -> #3 ; #4 (separate root)
	prs := []PullRequest{
		{Number: 1, State: StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: StateOpen, BaseRef: "b", HeadRef: "c"},
		{Number: 4, State: StateOpen, BaseRef: "main", HeadRef: "d"},
	}
	forest := BuildForest(prs, "main")

	// From a mid-tree PR, WholeTree returns its forest root's whole tree.
	got := WholeTree(forest, 3)
	eq(t, numbers(got), []int{1})
	eq(t, numbers(got[0].Children), []int{2})

	if WholeTree(forest, 99) != nil {
		t.Fatalf("absent PR should yield nil")
	}
}

func TestLiveRoots(t *testing.T) {
	// #1 MERGED (root) -> #2 OPEN -> #3 OPEN ; #4 OPEN (root) -> #5 DRAFT
	// #6 CLOSED (root) -> #7 OPEN
	prs := []PullRequest{
		{Number: 1, State: StateMerged, BaseRef: "main", HeadRef: "a"},
		// #1 is MERGED, so its head branch "a" is not "live" and branch topology
		// can't link #2 under it (same rule as the CLOSED case below); use an
		// explicit upstream link so #2 genuinely nests under merged #1 — this is
		// what exercises the "parent is MERGED -> live root" path.
		{Number: 2, State: StateOpen, BaseRef: "main", HeadRef: "b", Body: "upstream: #1"},
		{Number: 3, State: StateOpen, BaseRef: "b", HeadRef: "c"},
		{Number: 4, State: StateOpen, BaseRef: "main", HeadRef: "d"},
		{Number: 5, State: StateDraft, BaseRef: "d", HeadRef: "e"},
		{Number: 6, State: StateClosed, BaseRef: "main", HeadRef: "f"},
		// #6 is CLOSED, so its head branch "f" is not "live" (BuildForest.IsLive)
		// and branch topology can't link #7 under it; use an explicit upstream
		// link so #7 is genuinely nested under closed #6 in the built forest.
		{Number: 7, State: StateOpen, BaseRef: "main", HeadRef: "g", Body: "upstream: #6"},
	}
	forest := BuildForest(prs, "main")
	got := LiveRoots(forest)
	// #1,#4,#6 are forest roots; #2 sits under MERGED #1 so it is a live root.
	// #3 sits under OPEN #2, #5 under OPEN #4, #7 under CLOSED #6 -> not live roots.
	eq(t, numbers(got), []int{1, 2, 4, 6})
	for _, n := range got {
		if len(n.Children) != 0 {
			t.Fatalf("LiveRoots node #%d should be flat, has %d children", n.PR.Number, len(n.Children))
		}
	}
}
