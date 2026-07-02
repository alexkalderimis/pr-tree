package render

import (
	"strings"
	"testing"
)

func TestAnnotatePlanPlain(t *testing.T) {
	items := []AnnotateItem{
		{Number: 1, Title: "ROOT", Change: ChangeNone},
		{Number: 2, Title: "STEM", Change: ChangeInsert,
			OldBody: "Intro.", NewBody: "Intro.\n+links"},
	}
	got := AnnotatePlan(items, Options{Color: false})

	if !strings.Contains(got, "#1 (ROOT)  no change") {
		t.Errorf("missing no-change line:\n%s", got)
	}
	if !strings.Contains(got, "#2 (STEM)  insert links block") {
		t.Errorf("missing insert line:\n%s", got)
	}
	// The added line appears with a "+ " marker; unchanged context with "  ".
	if !strings.Contains(got, "+ +links") {
		t.Errorf("missing added diff line:\n%s", got)
	}
	if !strings.Contains(got, "  Intro.") {
		t.Errorf("missing context diff line:\n%s", got)
	}
	// Plain output must contain no ANSI escapes.
	if strings.Contains(got, "\x1b[") {
		t.Errorf("plain output contains ANSI escapes:\n%q", got)
	}
}

func TestAnnotatePlanColorMarksAddRemove(t *testing.T) {
	items := []AnnotateItem{
		{Number: 2, Title: "STEM", Change: ChangeUpdate,
			OldBody: "old line", NewBody: "new line"},
	}
	got := AnnotatePlan(items, Options{Color: true})
	// Removed line carries red (31), added line carries green (32).
	if !strings.Contains(got, "\x1b[31m- old line") {
		t.Errorf("removed line not red:\n%q", got)
	}
	if !strings.Contains(got, "\x1b[32m+ new line") {
		t.Errorf("added line not green:\n%q", got)
	}
}

func TestAnnotatePlanCollapsesContext(t *testing.T) {
	// 10 identical leading lines, then one changed line. Lines far from the
	// change must be collapsed behind an ellipsis, not all printed.
	var old, new strings.Builder
	for i := 0; i < 10; i++ {
		old.WriteString("line\n")
		new.WriteString("line\n")
	}
	old.WriteString("A")
	new.WriteString("B")
	items := []AnnotateItem{
		{Number: 1, Title: "X", Change: ChangeUpdate, OldBody: old.String(), NewBody: new.String()},
	}
	got := AnnotatePlan(items, Options{Color: false})
	if !strings.Contains(got, "…") {
		t.Errorf("expected collapsed-context ellipsis:\n%s", got)
	}
	if strings.Count(got, "  line") > 6 { // at most 3 context each side of the change
		t.Errorf("context not collapsed, too many context lines:\n%s", got)
	}
}

func TestAnnotatePlanEmptyOldBodyNoSpuriousDelete(t *testing.T) {
	items := []AnnotateItem{
		{Number: 1, Title: "X", Change: ChangeInsert, OldBody: "", NewBody: "block line 1\nblock line 2"},
	}
	got := AnnotatePlan(items, Options{Color: false})
	if strings.Contains(got, "- ") {
		t.Errorf("empty old body produced a spurious deletion line:\n%s", got)
	}
	if !strings.Contains(got, "+ block line 1") {
		t.Errorf("missing inserted line:\n%s", got)
	}
}

func TestAnnotatePlanTrailingNewlineNoSpuriousLine(t *testing.T) {
	// Bodies differ ONLY by an appended block; the shared prefix has a trailing
	// newline. The diff must not show a spurious blank +/- line from the split.
	items := []AnnotateItem{
		{Number: 1, Title: "X", Change: ChangeUpdate, OldBody: "Intro.\n", NewBody: "Intro.\nblock"},
	}
	got := AnnotatePlan(items, Options{Color: false})
	// The only added line should be "block"; there must be no bare "+ " or "- " (empty) lines.
	if strings.Contains(got, "+ \n") || strings.Contains(got, "- \n") {
		t.Errorf("spurious blank diff line from trailing newline:\n%q", got)
	}
	if !strings.Contains(got, "+ block") {
		t.Errorf("missing added block line:\n%s", got)
	}
}
