// Package render turns a PR forest into the textual tree output. It is pure.
package render

import (
	"strconv"
	"strings"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// ANSI SGR codes.
const (
	ansiReset     = "0"
	ansiBold      = "1"
	ansiDim       = "2"
	ansiUnderline = "4"
	ansiRed       = "31"
	ansiGreen     = "32"
	ansiYellow    = "33"
	ansiMagenta   = "35"
	ansiCyan      = "36"
)

// Options controls rendering details that depend on the active filter.
type Options struct {
	// ReviewPending lists PR numbers to annotate with "<== Review pending".
	ReviewPending map[int]bool
	// Color enables ANSI color output. When false, output is plain text.
	Color bool
}

// style wraps s in the given SGR codes when color is enabled; otherwise it
// returns s unchanged. This keeps Color:false output byte-identical to plain.
func style(s string, color bool, codes ...string) string {
	if !color || len(codes) == 0 {
		return s
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + s + "\x1b[" + ansiReset + "m"
}

// rearm wraps s in the given SGR codes so the whole run carries them. The codes
// are re-armed after each inner reset so they stay continuous across colored
// segments and the uncolored glue between them (a single outer wrap would be
// cancelled by the first segment's reset).
func rearm(s string, codes ...string) string {
	on := "\x1b[" + strings.Join(codes, ";") + "m"
	reset := "\x1b[" + ansiReset + "m"
	return on + strings.ReplaceAll(s, reset, reset+on) + reset
}

// statusCodes returns the SGR codes for a PR state, or nil for unknown states.
func statusCodes(state tree.State) []string {
	switch state {
	case tree.StateOpen:
		return []string{ansiGreen}
	case tree.StateDraft:
		return []string{ansiDim}
	case tree.StateMerged:
		return []string{ansiMagenta}
	case tree.StateClosed:
		return []string{ansiRed}
	default:
		return nil
	}
}

// Render returns the textual tree for the forest. Roots are flush-left; each
// deeper level is indented two spaces and prefixed with "└ ".
func Render(forest []*tree.Node, opts Options) string {
	var b strings.Builder
	for _, root := range forest {
		renderNode(&b, root, 0, opts)
	}
	return b.String()
}

func renderNode(b *strings.Builder, n *tree.Node, depth int, opts Options) {
	if depth > 0 {
		b.WriteString(strings.Repeat("  ", depth-1))
		b.WriteString(style("└ ", opts.Color, ansiDim))
	}
	b.WriteString(nodeLine(n.PR, opts))
	b.WriteByte('\n')
	for _, c := range n.Children {
		renderNode(b, c, depth+1, opts)
	}
}

// nodeLine formats "#N (title[, reviewer: @x], STATUS)[ <== Review pending]",
// applying color per segment when enabled.
func nodeLine(pr tree.PullRequest, opts Options) string {
	num := style("#"+strconv.Itoa(pr.Number), opts.Color, ansiCyan)

	parts := []string{style(pr.Title, opts.Color, ansiBold)}
	if len(pr.Reviewers) > 0 {
		ats := make([]string, len(pr.Reviewers))
		for i, r := range pr.Reviewers {
			ats[i] = style("@"+r, opts.Color, ansiYellow)
		}
		parts = append(parts, style("reviewer:", opts.Color, ansiDim)+" "+strings.Join(ats, ", "))
	}
	parts = append(parts, style(string(pr.State), opts.Color, statusCodes(pr.State)...))

	info := num + " (" + strings.Join(parts, ", ") + ")"
	pending := opts.ReviewPending[pr.Number]
	// A green check marks a PR with the reviews required to merge. Gated to OPEN:
	// reviewDecision can remain APPROVED on a merged PR fetched as context.
	approved := pr.State == tree.StateOpen && pr.ReviewDecision == tree.ReviewApproved
	// Decorate the info portion (not the connector or markers): underline on
	// review-pending lines, bold on approved lines. Both can apply and are
	// re-armed together. Color-gated: with color off, output stays plain.
	if opts.Color {
		var codes []string
		if pending {
			codes = append(codes, ansiUnderline)
		}
		if approved {
			codes = append(codes, ansiBold)
		}
		if len(codes) > 0 {
			info = rearm(info, codes...)
		}
	}
	if approved {
		info += " " + style("✓", opts.Color, ansiGreen)
	}
	if pending {
		info += " " + style("<== Review pending", opts.Color, ansiBold, ansiYellow)
	}
	return info
}
