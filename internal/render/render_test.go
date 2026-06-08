package render

import (
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func TestRender_TreeWithMarkers(t *testing.T) {
	forest := []*tree.Node{
		{
			PR: tree.PullRequest{Number: 1234, Title: "ROOT", State: tree.StateMerged, Reviewers: []string{"abc"}},
			Children: []*tree.Node{
				{
					PR: tree.PullRequest{Number: 1235, Title: "STEM", State: tree.StateOpen, Reviewers: []string{"xyz"}},
					Children: []*tree.Node{
						{PR: tree.PullRequest{Number: 1236, Title: "LEAF-1", State: tree.StateOpen, Reviewers: []string{"foo"}}},
						{PR: tree.PullRequest{Number: 1237, Title: "LEAF-2", State: tree.StateDraft, Reviewers: []string{"bar"}}},
					},
				},
			},
		},
	}

	got := Render(forest, Options{ReviewPending: map[int]bool{1235: true}})
	want := `#1234 (ROOT, reviewer: @abc, MERGED)
└ #1235 (STEM, reviewer: @xyz, OPEN) <== Review pending
  └ #1236 (LEAF-1, reviewer: @foo, OPEN)
  └ #1237 (LEAF-2, reviewer: @bar, DRAFT)
`
	if got != want {
		t.Fatalf("Render mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRender_NoReviewer(t *testing.T) {
	forest := []*tree.Node{
		{PR: tree.PullRequest{Number: 5, Title: "B", State: tree.StateOpen}},
	}
	got := Render(forest, Options{})
	want := "#5 (B, OPEN)\n"
	if got != want {
		t.Fatalf("Render mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}
