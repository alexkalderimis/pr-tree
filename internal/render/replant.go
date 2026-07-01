package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// replantTitleMax is the maximum rendered title length, in runes.
const replantTitleMax = 50

// Commit is a dropped commit shown in a replant plan: OID (shortened on render)
// and subject.
type Commit struct {
	OID     string
	Subject string
}

// ReplantPlanInput is the fully-resolved data for one replant plan view. It
// carries no git or network dependency: the command layer resolves the forest,
// the re-homing, and the dropped commits before calling ReplantPlan.
type ReplantPlanInput struct {
	Header       string            // e.g. "Replant plan for #5925 (title)"
	Target       *tree.Node        // the target PR with its descendant subtree
	Reparented   bool              // true → Before/After; false → single Stack:
	OldParent    *tree.PullRequest // Before root (when Reparented)
	NewBaseLabel string            // After root label, e.g. "master"
	Dropped      []Commit          // commits removed by the target's rebase
	DropVia      int               // parent PR number for the dropped header
	ForkWarning  string            // non-empty → shown instead of Dropped
	Footer       string            // trailing note (e.g. dry-run hint)
}

// ReplantPlan renders the before/after (or single) stack plus the dropped-commit
// list. Pure; colour is applied through the shared style helpers when
// opts.Color is set, so opts.Color==false output is plain text.
func ReplantPlan(in ReplantPlanInput, opts Options) string {
	var b strings.Builder
	if in.Header != "" {
		b.WriteString(in.Header)
		b.WriteString("\n\n")
	}

	targetPR := 0
	if in.Target != nil {
		targetPR = in.Target.PR.Number
	}

	if in.Reparented {
		b.WriteString("Before:\n")
		if in.OldParent != nil {
			b.WriteString(replantLine(0, *in.OldParent, targetPR, opts))
		}
		if in.Target != nil {
			writeReplantSubtree(&b, in.Target, 1, targetPR, opts)
		}
		b.WriteString("\nAfter:\n")
		b.WriteString("  " + in.NewBaseLabel + "\n")
		if in.Target != nil {
			writeReplantSubtree(&b, in.Target, 1, targetPR, opts)
		}
	} else {
		b.WriteString("Stack:\n")
		if in.Target != nil {
			writeReplantSubtree(&b, in.Target, 0, targetPR, opts)
		}
	}

	switch {
	case in.ForkWarning != "":
		b.WriteString("\n⚠ " + in.ForkWarning + "\n")
	case len(in.Dropped) > 0:
		noun := "commits"
		if len(in.Dropped) == 1 {
			noun = "commit"
		}
		b.WriteString(fmt.Sprintf("\nDropping %d %s already merged via #%d:\n", len(in.Dropped), noun, in.DropVia))
		for _, c := range in.Dropped {
			b.WriteString("  " + shortOID(c.OID) + "  " + c.Subject + "\n")
		}
	}

	if in.Footer != "" {
		b.WriteString(in.Footer)
		if !strings.HasSuffix(in.Footer, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// writeReplantSubtree renders node at the given depth then recurses, one line
// per node.
func writeReplantSubtree(b *strings.Builder, n *tree.Node, depth, targetPR int, opts Options) {
	b.WriteString(replantLine(depth, n.PR, targetPR, opts))
	for _, c := range n.Children {
		writeReplantSubtree(b, c, depth+1, targetPR, opts)
	}
}

// replantLine formats one node line: a two-space margin, a depth connector, then
// "#num branch  title[ (state)][  ← this PR]".
func replantLine(depth int, pr tree.PullRequest, targetPR int, opts Options) string {
	connector := ""
	if depth > 0 {
		connector = strings.Repeat("  ", depth-1) + style("└ ", opts.Color, ansiDim)
	}

	num := style("#"+strconv.Itoa(pr.Number), opts.Color, ansiCyan)
	branch := style(pr.HeadRef, opts.Color, ansiDim)
	label := num + " " + branch + "  " + truncateRunes(pr.Title, replantTitleMax)

	if pr.State != tree.StateOpen {
		if codes := statusCodes(pr.State); len(codes) > 0 {
			label += " " + style("("+strings.ToLower(string(pr.State))+")", opts.Color, codes...)
		}
	}
	if pr.Number == targetPR {
		label += "  " + style("← this PR", opts.Color, ansiBold)
	}
	return "  " + connector + label + "\n"
}

// truncateRunes shortens s to max runes, replacing the tail with "…".
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// shortOID returns the first 7 characters of an OID.
func shortOID(oid string) string {
	if len(oid) > 7 {
		return oid[:7]
	}
	return oid
}
