package main

import (
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func TestChooseSelector(t *testing.T) {
	cases := []struct {
		name                     string
		root, parentSet, treeSet bool
		want                     selector
		wantErr                  bool
	}{
		{"none", false, false, false, selNone, false},
		{"root", true, false, false, selRoot, false},
		{"parent", false, true, false, selParent, false},
		{"tree", false, false, true, selTree, false},
		{"root+parent", true, true, false, selNone, true},
		{"parent+tree", false, true, true, selNone, true},
		{"all three", true, true, true, selNone, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := chooseSelector(tc.root, tc.parentSet, tc.treeSet)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got selector %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParsePRNumber(t *testing.T) {
	for _, in := range []string{"1234", "#1234"} {
		n, err := parsePRNumber(in)
		if err != nil || n != 1234 {
			t.Fatalf("parsePRNumber(%q) = %d, %v", in, n, err)
		}
	}
	if _, err := parsePRNumber("abc"); err == nil {
		t.Fatal("expected error for non-numeric input")
	}
}

func TestPRForBranch(t *testing.T) {
	prs := []tree.PullRequest{
		{Number: 1, State: tree.StateOpen, HeadRef: "feat-a"},
		{Number: 2, State: tree.StateMerged, HeadRef: "feat-b"},
	}
	if n, ok := prForBranch("feat-a", prs); !ok || n != 1 {
		t.Fatalf("prForBranch(feat-a) = %d, %v", n, ok)
	}
	// Merged branch is not live -> not matched.
	if _, ok := prForBranch("feat-b", prs); ok {
		t.Fatal("merged PR branch should not match")
	}
	if _, ok := prForBranch("missing", prs); ok {
		t.Fatal("unknown branch should not match")
	}
}
