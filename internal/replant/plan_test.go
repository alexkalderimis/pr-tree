package replant

import (
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// steps returns the PR numbers of the given steps, in order.
func steps(ss []Step) []int {
	out := make([]int, len(ss))
	for i, s := range ss {
		out[i] = s.PR
	}
	return out
}

func TestPlan_RootMerged(t *testing.T) {
	// #1 squash-merged into main; #2 stacks on #1's branch "a"; #3 stacks on
	// #2's branch "b". Replanting from #1 should rebase #2 onto main (its parent
	// merged) then #3 onto "b" (its parent #2 was rebased, not merged), top-down.
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateMerged, BaseRef: "main", HeadRef: "a", Body: "upstream: #0"},
		{Number: 2, State: tree.StateOpen, BaseRef: "main", HeadRef: "b", Body: "upstream: #1"},
		{Number: 3, State: tree.StateOpen, BaseRef: "b", HeadRef: "c"},
	}
	forest := tree.BuildForest(prs, "main")

	plan, err := Plan(forest, 1, "main")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if got, want := steps(plan), []int{2, 3}; !equalInts(got, want) {
		t.Fatalf("order: got %v, want %v", got, want)
	}

	// #2's parent (#1) merged → rebase onto the default branch.
	if plan[0].NewBaseRef != "main" {
		t.Errorf("#2 NewBaseRef: got %q, want %q", plan[0].NewBaseRef, "main")
	}
	if !plan[0].ParentMerged || plan[0].ParentPR != 1 {
		t.Errorf("#2 parent: got pr=%d merged=%v, want pr=1 merged=true", plan[0].ParentPR, plan[0].ParentMerged)
	}

	// #3's parent (#2) was rebased, not merged → rebase onto #2's head branch.
	if plan[1].NewBaseRef != "b" {
		t.Errorf("#3 NewBaseRef: got %q, want %q", plan[1].NewBaseRef, "b")
	}
	if plan[1].ParentMerged || plan[1].ParentPR != 2 {
		t.Errorf("#3 parent: got pr=%d merged=%v, want pr=2 merged=false", plan[1].ParentPR, plan[1].ParentMerged)
	}
}

func TestPlan_IntermediateChange(t *testing.T) {
	// Nothing merged; #2 was amended. Replanting from #2 rebases only its
	// descendants (#3) onto #2's head branch. #1 (ancestor) is untouched.
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: tree.StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: tree.StateOpen, BaseRef: "b", HeadRef: "c"},
	}
	forest := tree.BuildForest(prs, "main")

	plan, err := Plan(forest, 2, "main")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if got, want := steps(plan), []int{3}; !equalInts(got, want) {
		t.Fatalf("order: got %v, want %v", got, want)
	}
	if plan[0].NewBaseRef != "b" || plan[0].ParentMerged {
		t.Errorf("#3 step: got base=%q merged=%v, want base=b merged=false", plan[0].NewBaseRef, plan[0].ParentMerged)
	}
	if plan[0].HeadRef != "c" {
		t.Errorf("#3 HeadRef: got %q, want %q", plan[0].HeadRef, "c")
	}
}

func TestPlan_TargetWithNoDescendants(t *testing.T) {
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateOpen, BaseRef: "main", HeadRef: "a"},
	}
	forest := tree.BuildForest(prs, "main")

	plan, err := Plan(forest, 1, "main")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan) != 0 {
		t.Fatalf("expected no steps, got %v", steps(plan))
	}
}

func TestPlan_TargetNotFound(t *testing.T) {
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateOpen, BaseRef: "main", HeadRef: "a"},
	}
	forest := tree.BuildForest(prs, "main")

	if _, err := Plan(forest, 99, "main"); err == nil {
		t.Fatal("expected an error for an unknown target PR, got nil")
	}
}

func TestPlan_BranchingDescendantsAreTopDown(t *testing.T) {
	// #1 merged; #2 and #3 both stack on it; #4 stacks on #2. Every parent must
	// appear before its child in the plan.
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateMerged, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: tree.StateOpen, BaseRef: "main", HeadRef: "b", Body: "upstream: #1"},
		{Number: 3, State: tree.StateOpen, BaseRef: "main", HeadRef: "c", Body: "upstream: #1"},
		{Number: 4, State: tree.StateOpen, BaseRef: "b", HeadRef: "d"},
	}
	forest := tree.BuildForest(prs, "main")

	plan, err := Plan(forest, 1, "main")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	pos := make(map[int]int)
	for i, s := range plan {
		pos[s.PR] = i
	}
	if _, ok := pos[4]; !ok {
		t.Fatalf("expected #4 in plan, got %v", steps(plan))
	}
	if pos[2] > pos[4] {
		t.Errorf("#2 must come before its child #4: got %v", steps(plan))
	}
}

func equalInts(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
