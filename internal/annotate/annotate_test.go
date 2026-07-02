package annotate

import (
	"strings"
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

func TestRenderBlock(t *testing.T) {
	got := RenderBlock(1234, []int{1236, 1237})
	want := "<!-- pr-tree:links -->\n### Stack\n\n" +
		"- **Upstream:** #1234\n" +
		"- **Downstream:** #1236, #1237\n" +
		"<!-- /pr-tree:links -->"
	if got != want {
		t.Fatalf("RenderBlock mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestRenderBlockRootAndLeaf(t *testing.T) {
	got := RenderBlock(0, nil)
	if !strings.Contains(got, "- **Upstream:** _(none — root)_") {
		t.Errorf("root upstream line missing: %q", got)
	}
	if !strings.Contains(got, "- **Downstream:** _(none — leaf)_") {
		t.Errorf("leaf downstream line missing: %q", got)
	}
}

// The rendered upstream line must stay parseable by tree.ParseUpstream so that
// list keeps reconstructing merged parents.
func TestRenderBlockParsesBackToUpstream(t *testing.T) {
	block := RenderBlock(42, []int{7})
	if got := tree.ParseUpstream(block); got != 42 {
		t.Fatalf("ParseUpstream(block) = %d, want 42", got)
	}
	// A root block must NOT yield a parent.
	if got := tree.ParseUpstream(RenderBlock(0, []int{7})); got != 0 {
		t.Fatalf("ParseUpstream(root block) = %d, want 0", got)
	}
}

func TestUpsertFreshAppend(t *testing.T) {
	block := RenderBlock(1, []int{2})
	got := Upsert("Original description.", block)
	want := "Original description.\n\n" + block
	if got != want {
		t.Fatalf("fresh upsert:\n got %q\nwant %q", got, want)
	}
}

func TestUpsertReplacesInPlace(t *testing.T) {
	old := RenderBlock(1, []int{2})
	body := "Intro.\n\n" + old + "\n\nOutro."
	newBlock := RenderBlock(1, []int{2, 3})
	got := Upsert(body, newBlock)
	want := "Intro.\n\n" + newBlock + "\n\nOutro."
	if got != want {
		t.Fatalf("in-place replace:\n got %q\nwant %q", got, want)
	}
}

func TestUpsertStripsFreeFormStackedOn(t *testing.T) {
	block := RenderBlock(1, []int{2})
	body := "Stacked on #1\n\nReal description."
	got := Upsert(body, block)
	if strings.Contains(got, "Stacked on #1") {
		t.Fatalf("free-form note not stripped: %q", got)
	}
	if !strings.Contains(got, "Real description.") {
		t.Fatalf("description body lost: %q", got)
	}
	if !strings.Contains(got, block) {
		t.Fatalf("block not inserted: %q", got)
	}
}

func TestUpsertDoesNotTouchProse(t *testing.T) {
	block := RenderBlock(1, nil)
	body := "This work is stacked on top of the parser rewrite."
	got := Upsert(body, block)
	if !strings.Contains(got, "This work is stacked on top of the parser rewrite.") {
		t.Fatalf("mid-sentence prose wrongly stripped: %q", got)
	}
}

func TestUpsertIsIdempotent(t *testing.T) {
	block := RenderBlock(1, []int{2})
	once := Upsert("Stacked on #1\n\nReal description.", block)
	twice := Upsert(once, block)
	if once != twice {
		t.Fatalf("not idempotent:\n once %q\ntwice %q", once, twice)
	}
}

func TestHasBlock(t *testing.T) {
	if HasBlock("no markers here") {
		t.Fatal("HasBlock false positive")
	}
	if !HasBlock("x\n" + RenderBlock(1, nil) + "\ny") {
		t.Fatal("HasBlock false negative")
	}
}
