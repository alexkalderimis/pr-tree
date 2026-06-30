package tree

import "testing"

func TestParseUpstream(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"plain", "upstream: #1234", 1234},
		{"list item", "Stack:\n- upstream: #42\n- downstream: #43", 42},
		{"no space", "upstream:#7", 7},
		{"absent", "no links here", 0},
		{"downstream only", "downstream: #99", 0},
		{"empty", "", 0},
		{"multiple", "upstream: #1\nupstream: #2", 1},
		// Human-written stack sections (not produced by annotate) must also be
		// understood, taking the first (immediate-parent) PR number on the line.
		{"human merge-first", "**Upstream (merge first):** #5924 → #5923 → #5922", 5924},
		{"human heading", "Upstream: #5924", 5924},
		{"human with downstream after", "**Upstream:** #12 → #11\n**Downstream:** #20", 12},
		// "upstream" must be a whole word: don't match "upstreaming:".
		{"not upstreaming", "upstreaming: #5", 0},
		// A "stream"-suffixed word that isn't upstream must not match.
		{"downstream not matched", "Downstream (merge later): #99", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseUpstream(c.body); got != c.want {
				t.Fatalf("ParseUpstream(%q) = %d, want %d", c.body, got, c.want)
			}
		})
	}
}
