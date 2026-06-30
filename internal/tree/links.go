package tree

import (
	"regexp"
	"strconv"
)

// upstreamRe matches an upstream reference on a single line, capturing the
// first PR number that follows. It accepts both the machine `upstream: #N`
// form and human-written stack headings such as
// `**Upstream (merge first):** #5924 → #5923`. The `\b` keeps "upstream" a
// whole word (so "upstreaming:" and "downstream:" never match), and requiring
// a colon before the `#` avoids matching prose that merely mentions "upstream".
// `[^#\n]*` is line-bounded, so on a multi-PR line the first number — the
// immediate parent — is the one captured.
var upstreamRe = regexp.MustCompile(`(?i)upstream\b[^#:\n]*:[^#\n]*#(\d+)`)

// ParseUpstream returns the immediate-parent PR number referenced by an upstream
// link in a PR body, or 0 if none is present. It understands both the
// `upstream: #N` form and human-written "Upstream: #N → …" stack headings.
func ParseUpstream(body string) int {
	m := upstreamRe.FindStringSubmatch(body)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}
