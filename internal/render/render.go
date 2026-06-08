// Package render turns a PR forest into the textual tree output. It is pure.
package render

import (
	"strconv"
	"strings"

	"github.com/alexkalderimis/pr-tree/internal/tree"
)

// Options controls rendering details that depend on the active filter.
type Options struct {
	// ReviewPending lists PR numbers to annotate with "<== Review pending".
	ReviewPending map[int]bool
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
		b.WriteString("└ ")
	}
	b.WriteString(nodeLine(n.PR, opts))
	b.WriteByte('\n')
	for _, c := range n.Children {
		renderNode(b, c, depth+1, opts)
	}
}

// nodeLine formats "#N (title[, reviewer: @x], STATUS)[ <== Review pending]".
func nodeLine(pr tree.PullRequest, opts Options) string {
	parts := []string{pr.Title}
	if len(pr.Reviewers) > 0 {
		ats := make([]string, len(pr.Reviewers))
		for i, r := range pr.Reviewers {
			ats[i] = "@" + r
		}
		parts = append(parts, "reviewer: "+strings.Join(ats, ", "))
	}
	parts = append(parts, string(pr.State))

	line := "#" + strconv.Itoa(pr.Number) + " (" + strings.Join(parts, ", ") + ")"
	if opts.ReviewPending[pr.Number] {
		line += " <== Review pending"
	}
	return line
}
