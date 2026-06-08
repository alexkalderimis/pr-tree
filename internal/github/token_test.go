package github

import "testing"

func TestTokenFromEnv(t *testing.T) {
	env := map[string]string{"GH_TOKEN": "abc"}
	getenv := func(k string) string { return env[k] }
	if got := tokenFromEnv(getenv); got != "abc" {
		t.Fatalf("tokenFromEnv = %q, want abc", got)
	}

	env = map[string]string{"GITHUB_TOKEN": "xyz"}
	if got := tokenFromEnv(getenv); got != "xyz" {
		t.Fatalf("tokenFromEnv = %q, want xyz", got)
	}

	env = map[string]string{}
	if got := tokenFromEnv(getenv); got != "" {
		t.Fatalf("tokenFromEnv = %q, want empty", got)
	}
}
