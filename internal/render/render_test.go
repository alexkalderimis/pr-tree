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

func TestRender_Colored(t *testing.T) {
	forest := []*tree.Node{
		{
			PR: tree.PullRequest{Number: 1, Title: "ROOT", State: tree.StateMerged},
			Children: []*tree.Node{
				{PR: tree.PullRequest{Number: 2, Title: "LEAF", State: tree.StateOpen, Reviewers: []string{"bob"}}},
			},
		},
	}

	got := Render(forest, Options{Color: true, ReviewPending: map[int]bool{2: true}})

	// Root #1: not review-pending → no underline.
	root := "\x1b[36m#1\x1b[0m (\x1b[1mROOT\x1b[0m, \x1b[35mMERGED\x1b[0m)\n"
	// Child #2: review-pending → the info portion (#num … closing paren) is
	// underlined, with the underline re-armed after each inner reset so it stays
	// continuous across colored segments and the uncolored glue. The dim
	// connector and the bold-yellow marker are NOT underlined.
	child := "\x1b[2m└ \x1b[0m" +
		"\x1b[4m\x1b[36m#2\x1b[0m\x1b[4m (\x1b[1mLEAF\x1b[0m\x1b[4m, \x1b[2mreviewer:\x1b[0m\x1b[4m \x1b[33m@bob\x1b[0m\x1b[4m, \x1b[32mOPEN\x1b[0m\x1b[4m)\x1b[0m" +
		" \x1b[1;33m<== Review pending\x1b[0m\n"
	want := root + child
	if got != want {
		t.Fatalf("colored render mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestRender_ApprovedCheck_ColorOn(t *testing.T) {
	forest := []*tree.Node{
		{PR: tree.PullRequest{Number: 1, Title: "ROOT", State: tree.StateOpen, ReviewDecision: tree.ReviewApproved}},
	}
	got := Render(forest, Options{Color: true})
	want := "\x1b[36m#1\x1b[0m (\x1b[1mROOT\x1b[0m, \x1b[32mOPEN\x1b[0m) \x1b[32m✓\x1b[0m\n"
	if got != want {
		t.Fatalf("approved check (color):\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestRender_ApprovedCheck_ColorOff(t *testing.T) {
	forest := []*tree.Node{
		{PR: tree.PullRequest{Number: 1, Title: "ROOT", State: tree.StateOpen, ReviewDecision: tree.ReviewApproved}},
	}
	got := Render(forest, Options{})
	want := "#1 (ROOT, OPEN) ✓\n"
	if got != want {
		t.Fatalf("approved check (plain):\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestRender_ApprovedCheck_NotOnMerged(t *testing.T) {
	// reviewDecision can stay APPROVED on a merged PR (fetched as context);
	// the "ready to merge" check must not show on it.
	forest := []*tree.Node{
		{PR: tree.PullRequest{Number: 2, Title: "OLD", State: tree.StateMerged, ReviewDecision: tree.ReviewApproved}},
	}
	got := Render(forest, Options{Color: true})
	want := "\x1b[36m#2\x1b[0m (\x1b[1mOLD\x1b[0m, \x1b[35mMERGED\x1b[0m)\n"
	if got != want {
		t.Fatalf("merged should have no check:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestRender_ApprovedAndReviewPending_Order(t *testing.T) {
	// Order on the line: underlined info, then green check, then bold-yellow marker.
	forest := []*tree.Node{
		{PR: tree.PullRequest{Number: 3, Title: "P", State: tree.StateOpen, ReviewDecision: tree.ReviewApproved}},
	}
	got := Render(forest, Options{Color: true, ReviewPending: map[int]bool{3: true}})
	want := "\x1b[4m\x1b[36m#3\x1b[0m\x1b[4m (\x1b[1mP\x1b[0m\x1b[4m, \x1b[32mOPEN\x1b[0m\x1b[4m)\x1b[0m" +
		" \x1b[32m✓\x1b[0m" +
		" \x1b[1;33m<== Review pending\x1b[0m\n"
	if got != want {
		t.Fatalf("approved+pending order:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestRender_NoUnderlineWhenColorOff(t *testing.T) {
	// Underline is color-gated: with Color:false a review-pending line stays
	// plain (byte-identical to before underline existed).
	forest := []*tree.Node{
		{PR: tree.PullRequest{Number: 7, Title: "C", State: tree.StateOpen}},
	}
	got := Render(forest, Options{ReviewPending: map[int]bool{7: true}})
	want := "#7 (C, OPEN) <== Review pending\n"
	if got != want {
		t.Fatalf("expected no underline when color off:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}
