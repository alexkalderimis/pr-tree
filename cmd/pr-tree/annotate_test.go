package main

import (
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/annotate"
	"github.com/alexkalderimis/pr-tree/internal/render"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func TestAnnotateItemsChangeClassification(t *testing.T) {
	updates := []annotate.Update{
		{PR: tree.PullRequest{Number: 1, Title: "ROOT"}, OldBody: "x", NewBody: "x", Changed: false},
		{PR: tree.PullRequest{Number: 2, Title: "NEW"}, OldBody: "no block", NewBody: "no block\n" + annotate.MarkerStart, Changed: true},
		{PR: tree.PullRequest{Number: 3, Title: "UPD"}, OldBody: annotate.RenderBlock(1, nil), NewBody: annotate.RenderBlock(1, []int{4}), Changed: true},
	}
	items := annotateItems(updates)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Change != render.ChangeNone {
		t.Errorf("#1 change = %v, want None", items[0].Change)
	}
	if items[1].Change != render.ChangeInsert {
		t.Errorf("#2 change = %v, want Insert (old body had no block)", items[1].Change)
	}
	if items[2].Change != render.ChangeUpdate {
		t.Errorf("#3 change = %v, want Update (old body had a block)", items[2].Change)
	}
	if items[2].Title != "UPD" || items[2].Number != 3 {
		t.Errorf("#3 fields wrong: %+v", items[2])
	}
}
