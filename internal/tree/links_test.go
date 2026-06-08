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
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseUpstream(c.body); got != c.want {
				t.Fatalf("ParseUpstream(%q) = %d, want %d", c.body, got, c.want)
			}
		})
	}
}
