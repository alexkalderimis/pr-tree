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
	return BuildForest(prs)
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
