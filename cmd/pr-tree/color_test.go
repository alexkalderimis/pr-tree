package main

import "testing"

func TestColorEnabled(t *testing.T) {
	cases := []struct {
		name     string
		noColor  bool
		noColEnv string
		isTTY    bool
		want     bool
	}{
		{"tty, nothing set", false, "", true, true},
		{"not a tty", false, "", false, false},
		{"--no-color overrides tty", true, "", true, false},
		{"NO_COLOR set overrides tty", false, "1", true, false},
		{"NO_COLOR empty is ignored", false, "", true, true},
		{"NO_COLOR disables independent of flag and tty", false, "1", false, false},
		{"all off", true, "1", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := colorEnabled(c.noColor, c.noColEnv, c.isTTY); got != c.want {
				t.Fatalf("colorEnabled(%v,%q,%v) = %v, want %v", c.noColor, c.noColEnv, c.isTTY, got, c.want)
			}
		})
	}
}
