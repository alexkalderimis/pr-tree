// Package render turns a PR forest into the textual tree output. It is pure.
package render

import (
	"strconv"
	"strings"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// ANSI SGR codes.
const (
	ansiReset   = "0"
	ansiBold    = "1"
	ansiDim     = "2"
	ansiRed     = "31"
	ansiGreen   = "32"
	ansiYellow  = "33"
	ansiMagenta = "35"
	ansiCyan    = "36"
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

	line := num + " (" + strings.Join(parts, ", ") + ")"
	if opts.ReviewPending[pr.Number] {
		line += " " + style("<== Review pending", opts.Color, ansiBold, ansiYellow)
	}
	return line
}
