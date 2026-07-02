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

// stackedOnRe matches a human-written "stacked on: #N" note, capturing the
// first PR number that follows. The colon is required, which keeps this a
// deliberate label: prose that merely mentions being "stacked on #N" (no colon)
// is not matched, and "stacked onto:" is not "stacked on:". As with upstreamRe,
// `\s*#(\d+)` takes the first number on a multi-PR line — the immediate parent.
var stackedOnRe = regexp.MustCompile(`(?i)\bstacked on:\s*#(\d+)`)

// ParseUpstream returns the immediate-parent PR number referenced by an upstream
// link in a PR body, or 0 if none is present. It understands the `upstream: #N`
// form, human-written "Upstream: #N → …" stack headings, and human "stacked on:
// #N" notes. An explicit `upstream:` link takes precedence over a `stacked on:`
// note when both are present.
func ParseUpstream(body string) int {
	if n := firstRef(upstreamRe, body); n != 0 {
		return n
	}
	return firstRef(stackedOnRe, body)
}

// firstRef returns the first PR number captured by re in body, or 0.
func firstRef(re *regexp.Regexp, body string) int {
	m := re.FindStringSubmatch(body)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}
