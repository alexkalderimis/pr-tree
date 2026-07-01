package main

import (
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/replant"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func TestBuildReplantView_ReparentedWithDrop(t *testing.T) {
	g, _, qOID, masterTip := buildDriftRepo(t)

	// #1 merged parent (head no longer on the branch), #2 target on its own branch.
	byNum := map[int]tree.PullRequest{
		1: {Number: 1, HeadRef: "pr1", Title: "parent", State: tree.StateMerged, HeadOID: masterTip},
		2: {Number: 2, HeadRef: "child", Title: "own work", State: tree.StateOpen, HeadOID: qOID},
	}
	forest := []*tree.Node{
		{PR: byNum[1], Children: []*tree.Node{{PR: byNum[2]}}},
	}
	plan := []replant.Step{
		{PR: 2, HeadRef: "child", NewBaseRef: "master", ParentPR: 1, ParentMerged: true, TargetSelf: true},
	}
	merged := subjectSet([]string{"feat: parent work"}, "")

	in := buildReplantView(g, byNum, forest, "master", 2, 0, plan, merged, "")

	if !in.Reparented || in.OldParent == nil || in.OldParent.Number != 1 {
		t.Fatalf("expected reparented view with old parent #1, got %+v", in)
	}
	if in.NewBaseLabel != "master" {
		t.Errorf("NewBaseLabel = %q, want master", in.NewBaseLabel)
	}
	if in.Target == nil || in.Target.PR.Number != 2 {
		t.Fatalf("target node = %+v, want #2", in.Target)
	}
	if in.DropVia != 1 {
		t.Errorf("DropVia = %d, want 1", in.DropVia)
	}
	// The already-merged "feat: parent work" commit is dropped; "own work" is kept.
	if len(in.Dropped) == 0 {
		t.Fatal("expected at least one dropped commit")
	}
	for _, c := range in.Dropped {
		if c.Subject == "feat: own work" {
			t.Errorf("own work must not be dropped: %+v", in.Dropped)
		}
	}
}

func TestBuildReplantView_MovedOpenParentDrops(t *testing.T) {
	// Open parent whose recorded head (masterTip) is NOT on the child branch
	// (moved/rewritten) — collapsed Stack (not reparented), but still sheds the
	// already-present commit. Exercises the origin-qualified base for the open case.
	g, _, qOID, masterTip := buildDriftRepo(t)
	byNum := map[int]tree.PullRequest{
		1: {Number: 1, HeadRef: "pr1", Title: "parent", State: tree.StateOpen, HeadOID: masterTip},
		2: {Number: 2, HeadRef: "child", Title: "own work", State: tree.StateOpen, HeadOID: qOID},
	}
	forest := []*tree.Node{{PR: byNum[1], Children: []*tree.Node{{PR: byNum[2]}}}}
	plan := []replant.Step{
		{PR: 2, HeadRef: "child", NewBaseRef: "pr1", ParentPR: 1, ParentMerged: false, TargetSelf: true},
	}
	merged := subjectSet([]string{"feat: parent work"}, "")

	in := buildReplantView(g, byNum, forest, "master", 2, 0, plan, merged, "")

	if in.Reparented {
		t.Error("open parent must render a collapsed Stack, not Before/After")
	}
	if len(in.Dropped) == 0 {
		t.Fatal("expected a dropped commit for the moved open parent")
	}
	for _, c := range in.Dropped {
		if c.Subject == "feat: own work" {
			t.Errorf("own work must not be dropped: %+v", in.Dropped)
		}
	}
}

func TestBuildReplantView_CollapsedWhenNoTargetStep(t *testing.T) {
	g, _, qOID, _ := buildDriftRepo(t)
	byNum := map[int]tree.PullRequest{
		2: {Number: 2, HeadRef: "child", Title: "own work", State: tree.StateOpen, HeadOID: qOID},
	}
	forest := []*tree.Node{{PR: byNum[2]}}
	// No TargetSelf step (target is a root restacking descendants).
	plan := []replant.Step{{PR: 3, HeadRef: "d", NewBaseRef: "child", ParentPR: 2}}

	in := buildReplantView(g, byNum, forest, "master", 2, 0, plan, nil, "")
	if in.Reparented {
		t.Errorf("expected collapsed (non-reparented) view")
	}
	if len(in.Dropped) != 0 || in.ForkWarning != "" {
		t.Errorf("expected no dropped list / warning, got dropped=%v warn=%q", in.Dropped, in.ForkWarning)
	}
}
