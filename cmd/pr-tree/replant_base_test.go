package main

import (
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/replant"
)

func TestBaseRef(t *testing.T) {
	const parentOID = "abc123"
	cases := []struct {
		name string
		step replant.Step
		want string
	}{
		{
			name: "merged parent rebases onto origin default",
			step: replant.Step{ParentMerged: true},
			want: "origin/main",
		},
		{
			name: "target step with open parent rebases onto parent origin head",
			step: replant.Step{TargetSelf: true, NewBaseRef: "feature-a"},
			want: parentOID,
		},
		{
			name: "descendant with open parent rebases onto local parent branch",
			step: replant.Step{NewBaseRef: "feature-a"},
			want: "feature-a",
		},
		{
			name: "merged wins even for the target step",
			step: replant.Step{TargetSelf: true, ParentMerged: true, NewBaseRef: "feature-a"},
			want: "origin/main",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := baseRef("main", tc.step, parentOID); got != tc.want {
				t.Errorf("baseRef = %q, want %q", got, tc.want)
			}
		})
	}
}
