package main

import (
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func TestInjectParentOverride(t *testing.T) {
	prs := []tree.PullRequest{
		{Number: 5925, Body: "## What\nsome text\n**Downstream:** none"},
		{Number: 5926, Body: "untouched"},
	}

	injectParentOverride(prs, 5925, 5924)

	// The override is now the parent the forest builder will see for the target.
	if got := tree.ParseUpstream(prs[0].Body); got != 5924 {
		t.Fatalf("after override, ParseUpstream(target body) = %d, want 5924", got)
	}
	// Other PRs are left alone.
	if prs[1].Body != "untouched" {
		t.Errorf("non-target body was mutated: %q", prs[1].Body)
	}
}

func TestInjectParentOverride_BeatsExistingUpstream(t *testing.T) {
	// The target already carries an (inferred-wrong) upstream link; the explicit
	// override must take precedence over it.
	prs := []tree.PullRequest{
		{Number: 5925, Body: "**Upstream:** #999 → #998"},
	}

	injectParentOverride(prs, 5925, 5924)

	if got := tree.ParseUpstream(prs[0].Body); got != 5924 {
		t.Fatalf("override should win over existing upstream: ParseUpstream = %d, want 5924", got)
	}
}
