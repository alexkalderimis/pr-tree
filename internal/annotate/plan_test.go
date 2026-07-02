package annotate

import (
	"strings"
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// leaf builds a childless node.
func leaf(num int, body string) *tree.Node {
	return &tree.Node{PR: tree.PullRequest{Number: num, Body: body}}
}

func TestPlanComputesParentsAndChildren(t *testing.T) {
	// 1 -> 2 -> 3, and 1 -> 4
	n3 := leaf(3, "")
	n2 := &tree.Node{PR: tree.PullRequest{Number: 2}, Children: []*tree.Node{n3}}
	n4 := leaf(4, "")
	n1 := &tree.Node{PR: tree.PullRequest{Number: 1}, Children: []*tree.Node{n2, n4}}

	got := Plan([]*tree.Node{n1})
	if len(got) != 4 {
		t.Fatalf("got %d updates, want 4", len(got))
	}
	byNum := map[int]Update{}
	for _, u := range got {
		byNum[u.PR.Number] = u
	}
	if byNum[1].Parent != 0 {
		t.Errorf("#1 parent = %d, want 0 (root)", byNum[1].Parent)
	}
	if got := byNum[1].Children; len(got) != 2 || got[0] != 2 || got[1] != 4 {
		t.Errorf("#1 children = %v, want [2 4]", got)
	}
	if byNum[2].Parent != 1 {
		t.Errorf("#2 parent = %d, want 1", byNum[2].Parent)
	}
	if got := byNum[2].Children; len(got) != 1 || got[0] != 3 {
		t.Errorf("#2 children = %v, want [3]", got)
	}
	if byNum[3].Parent != 2 || len(byNum[3].Children) != 0 {
		t.Errorf("#3 = parent %d children %v, want parent 2, no children", byNum[3].Parent, byNum[3].Children)
	}
}

func TestPlanChangedFlag(t *testing.T) {
	// A leaf root already carrying the correct block must be unchanged.
	correct := Upsert("Intro.", RenderBlock(0, nil))
	got := Plan([]*tree.Node{leaf(1, correct)})
	if got[0].Changed {
		t.Fatalf("expected no change, got NewBody %q", got[0].NewBody)
	}

	// A node whose body lacks the block must be Changed and gain it.
	got = Plan([]*tree.Node{leaf(1, "Intro.")})
	if !got[0].Changed {
		t.Fatal("expected Changed=true for a body without a block")
	}
	if !strings.Contains(got[0].NewBody, MarkerStart) {
		t.Fatalf("NewBody missing block: %q", got[0].NewBody)
	}
}
