// Package annotate renders and upserts the machine-managed "links" block that
// pr-tree writes into PR descriptions to make a stack navigable in GitHub. It
// performs no I/O.
package annotate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Marker comments delimit the managed block. They render invisibly in GitHub
// and let repeated runs find and replace the block precisely.
const (
	MarkerStart = "<!-- pr-tree:links -->"
	MarkerEnd   = "<!-- /pr-tree:links -->"
)

// markerRe matches an existing managed block, non-greedy across newlines.
var markerRe = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(MarkerStart) + `.*?` + regexp.QuoteMeta(MarkerEnd))

// stackedOnRe matches a standalone free-form stacking note that another tool or
// a human may have written, e.g. "Stacked on #1", "**Stacked on** #1",
// "Stacked on top of #1", optionally block-quoted. It is line-anchored and
// whole-line (the line must *begin* with "stacked on" after optional
// quote/bold/whitespace), so ordinary prose that merely mentions the phrase
// mid-sentence is never touched. The trailing \n? consumes the line's newline
// so no blank line is left behind.
var stackedOnRe = regexp.MustCompile(`(?im)^[ \t]*>?[ \t]*\*{0,2}stacked on(?: top of)?\b.*\n?`)

// multiNewlineRe collapses runs of 3+ newlines to a single blank line.
var multiNewlineRe = regexp.MustCompile(`\n{3,}`)

// RenderBlock renders the marker-wrapped links block for a node with the given
// immediate parent (0 = root) and immediate child PR numbers (empty = leaf).
func RenderBlock(parent int, children []int) string {
	var up string
	if parent == 0 {
		up = "_(none — root)_"
	} else {
		up = "#" + strconv.Itoa(parent)
	}

	var down string
	if len(children) == 0 {
		down = "_(none — leaf)_"
	} else {
		refs := make([]string, len(children))
		for i, c := range children {
			refs[i] = "#" + strconv.Itoa(c)
		}
		down = strings.Join(refs, ", ")
	}

	return fmt.Sprintf("%s\n### Stack\n\n- **Upstream:** %s\n- **Downstream:** %s\n%s",
		MarkerStart, up, down, MarkerEnd)
}

// HasBlock reports whether body already contains a managed links block.
func HasBlock(body string) bool {
	return markerRe.MatchString(body)
}

// Upsert returns body with prior stacking notes removed and block inserted:
// an existing managed block is replaced in place (preserving its position, so
// repeated runs are byte-stable); otherwise block is appended after a blank
// line. Free-form "stacked on" notes are always stripped so block supersedes
// them. The result is normalised (no 3+ newline runs, no leading/trailing
// blank lines) so re-running with an unchanged tree is a no-op.
func Upsert(body, block string) string {
	body = stackedOnRe.ReplaceAllString(body, "")
	if markerRe.MatchString(body) {
		body = markerRe.ReplaceAllLiteralString(body, block)
	} else {
		body = strings.TrimRight(body, " \t\n")
		if body == "" {
			body = block
		} else {
			body = body + "\n\n" + block
		}
	}
	return normalize(body)
}

// normalize collapses 3+ newline runs to a blank line and trims leading and
// trailing blank lines. It is idempotent.
func normalize(s string) string {
	s = multiNewlineRe.ReplaceAllString(s, "\n\n")
	return strings.Trim(s, "\n")
}
