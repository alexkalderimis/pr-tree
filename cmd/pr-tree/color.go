package main

// colorEnabled decides whether to emit ANSI color. Color is on only when stdout
// is a terminal and neither the --no-color flag nor the NO_COLOR environment
// variable (non-empty, per https://no-color.org) disables it.
func colorEnabled(noColorFlag bool, noColorEnv string, isTTY bool) bool {
	if noColorFlag || noColorEnv != "" || !isTTY {
		return false
	}
	return true
}
