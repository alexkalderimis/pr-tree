package render

import (
	"strings"
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func targetSubtree() *tree.Node {
	return &tree.Node{
		PR: tree.PullRequest{Number: 5925, HeadRef: "pr4", Title: "Make the surface default-org-aware", State: tree.StateOpen},
		Children: []*tree.Node{
			{PR: tree.PullRequest{Number: 5930, HeadRef: "pr5", Title: "Token hardening", State: tree.StateOpen}},
		},
	}
}

func TestReplantPlan_Reparented(t *testing.T) {
	oldParent := tree.PullRequest{Number: 5924, HeadRef: "pr3", Title: "Route transfer", State: tree.StateMerged}
	in := ReplantPlanInput{
		Header:       "Replant plan for #5925 (Make the surface default-org-aware)",
		Target:       targetSubtree(),
		Reparented:   true,
		OldParent:    &oldParent,
		NewBaseLabel: "master",
		Dropped:      []Commit{{OID: "13ced8b1122", Subject: "foundation"}, {OID: "55880e1339a", Subject: "route transfer"}},
		DropVia:      5924,
		Footer:       "\n(dry-run: pass --apply to execute)",
	}
	want := `Replant plan for #5925 (Make the surface default-org-aware)

Before:
  #5924 pr3  Route transfer (merged)
  └ #5925 pr4  Make the surface default-org-aware  ← this PR
    └ #5930 pr5  Token hardening

After:
  master
  └ #5925 pr4  Make the surface default-org-aware  ← this PR
    └ #5930 pr5  Token hardening

Dropping 2 commits already merged via #5924:
  13ced8b  foundation
  55880e1  route transfer

(dry-run: pass --apply to execute)
`
	if got := ReplantPlan(in, Options{}); got != want {
		t.Fatalf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestReplantPlan_Collapsed(t *testing.T) {
	in := ReplantPlanInput{
		Header: "Replant plan for #5925 (Make the surface default-org-aware)",
		Target: targetSubtree(),
		Footer: "\n(dry-run: pass --apply to execute)",
	}
	want := `Replant plan for #5925 (Make the surface default-org-aware)

Stack:
  #5925 pr4  Make the surface default-org-aware  ← this PR
  └ #5930 pr5  Token hardening

(dry-run: pass --apply to execute)
`
	if got := ReplantPlan(in, Options{}); got != want {
		t.Fatalf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestReplantPlan_ForkWarning(t *testing.T) {
	oldParent := tree.PullRequest{Number: 5924, HeadRef: "pr3", Title: "Route transfer", State: tree.StateMerged}
	in := ReplantPlanInput{
		Header:       "Replant plan for #5925 (x)",
		Target:       targetSubtree(),
		Reparented:   true,
		OldParent:    &oldParent,
		NewBaseLabel: "master",
		ForkWarning:  "can't tell which commits are already merged — re-run with --keep N",
	}
	got := ReplantPlan(in, Options{})
	if !strings.Contains(got, "⚠ can't tell which commits are already merged — re-run with --keep N") {
		t.Fatalf("missing fork warning:\n%s", got)
	}
	if strings.Contains(got, "Dropping") {
		t.Fatalf("should not show a dropped list when fork is unknown:\n%s", got)
	}
}

func TestReplantPlan_TruncatesLongTitle(t *testing.T) {
	long := strings.Repeat("a", 60)
	in := ReplantPlanInput{
		Target: &tree.Node{PR: tree.PullRequest{Number: 1, HeadRef: "b", Title: long, State: tree.StateOpen}},
	}
	got := ReplantPlan(in, Options{})
	want := strings.Repeat("a", 49) + "…"
	if !strings.Contains(got, want) {
		t.Fatalf("title not truncated to 50 runes:\n%s", got)
	}
	if strings.Contains(got, strings.Repeat("a", 51)) {
		t.Fatalf("title exceeded 50 runes:\n%s", got)
	}
}
