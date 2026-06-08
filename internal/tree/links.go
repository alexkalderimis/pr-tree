package tree

import (
	"regexp"
	"strconv"
)

var upstreamRe = regexp.MustCompile(`(?i)upstream:\s*#(\d+)`)

// ParseUpstream returns the PR number referenced by an `upstream: #N` link in
// a PR body, or 0 if none is present.
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
