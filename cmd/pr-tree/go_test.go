package main

import (
	"strings"
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func linear() []*tree.Node {
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: tree.StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: tree.StateOpen, BaseRef: "b", HeadRef: "c"},
	}
	return tree.BuildForest(prs, "main")
}

// #1 -> #2 -> #3 and #1 -> #4
func branching() []*tree.Node {
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateOpen, BaseRef: "main", HeadRef: "a"},
		{Number: 2, State: tree.StateOpen, BaseRef: "a", HeadRef: "b"},
		{Number: 3, State: tree.StateOpen, BaseRef: "b", HeadRef: "c"},
		{Number: 4, State: tree.StateOpen, BaseRef: "a", HeadRef: "d"},
	}
	return tree.BuildForest(prs, "main")
}

func TestResolveUp(t *testing.T) {
	f := linear()
	res, err := resolve("up", f, tree.Find(f, 2), "main")
	if err != nil || res.branch != "a" || res.prNum != 1 {
		t.Fatalf("up from #2 = %+v, err=%v; want branch a / prNum 1", res, err)
	}
	// From a root, up goes to the default branch.
	res, err = resolve("up", f, tree.Find(f, 1), "main")
	if err != nil || res.branch != "main" || res.prNum != 0 {
		t.Fatalf("up from root #1 = %+v, err=%v; want branch main / prNum 0", res, err)
	}
}

func TestResolveDown(t *testing.T) {
	f := branching()
	// No children -> error.
	if _, err := resolve("down", f, tree.Find(f, 3), "main"); err == nil {
		t.Fatalf("down from leaf #3 should error")
	}
	// One child -> direct.
	res, err := resolve("down", f, tree.Find(f, 2), "main")
	if err != nil || res.branch != "c" || len(res.candidates) != 0 {
		t.Fatalf("down from #2 = %+v, err=%v; want branch c, no candidates", res, err)
	}
	// Multiple children -> candidates.
	res, err = resolve("down", f, tree.Find(f, 1), "main")
	if err != nil || res.branch != "" || len(res.candidates) != 2 {
		t.Fatalf("down from #1 = %+v, err=%v; want 2 candidates", res, err)
	}
}

func TestResolveRoot(t *testing.T) {
	f := linear()
	res, err := resolve("root", f, tree.Find(f, 3), "main")
	if err != nil || res.branch != "a" || res.prNum != 1 {
		t.Fatalf("root from #3 = %+v, err=%v; want branch a / prNum 1", res, err)
	}
}

func TestResolveLeaf(t *testing.T) {
	f := branching()
	// Multiple leaves under #1 -> candidates.
	res, err := resolve("leaf", f, tree.Find(f, 1), "main")
	if err != nil || len(res.candidates) != 2 {
		t.Fatalf("leaf from #1 = %+v, err=%v; want 2 candidates", res, err)
	}
	// Already a leaf -> direct to itself.
	res, err = resolve("leaf", f, tree.Find(f, 3), "main")
	if err != nil || res.branch != "c" || res.prNum != 3 {
		t.Fatalf("leaf from #3 = %+v, err=%v; want branch c / prNum 3", res, err)
	}
}

func TestMatchCurrentPR(t *testing.T) {
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateMerged, HeadRef: "a"},
		{Number: 2, State: tree.StateOpen, HeadRef: "b"},
	}
	if pr, ok := matchCurrentPR("b", prs); !ok || pr.Number != 2 {
		t.Fatalf("match b = %v,%v; want #2,true", pr, ok)
	}
	if _, ok := matchCurrentPR("a", prs); ok {
		t.Fatalf("merged PR branch a should not match")
	}
	if _, ok := matchCurrentPR("zzz", prs); ok {
		t.Fatalf("unknown branch should not match")
	}
}

func TestChooseNode(t *testing.T) {
	f := branching()
	nodes := tree.Find(f, 1).Children // #2, #4

	// Invalid line is skipped; second line "2" picks the 2nd node (#4).
	var out strings.Builder
	got, err := chooseNode(strings.NewReader("abc\n2\n"), &out, "Pick:", nodes)
	if err != nil || got.PR.Number != 4 {
		t.Fatalf("chooseNode = %v, err=%v; want #4", got, err)
	}
	if !strings.Contains(out.String(), "1) #2") || !strings.Contains(out.String(), "2) #4") {
		t.Fatalf("numbered list missing from output:\n%s", out.String())
	}

	// EOF with no valid choice -> error.
	if _, err := chooseNode(strings.NewReader(""), &strings.Builder{}, "Pick:", nodes); err == nil {
		t.Fatalf("empty input should error")
	}
}
