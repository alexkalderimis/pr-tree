// Package github discovers credentials and queries the GitHub GraphQL API.
package github

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// tokenFromEnv returns a token from GH_TOKEN, then GITHUB_TOKEN.
func tokenFromEnv(getenv func(string) string) string {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// Token discovers a GitHub token: first `gh auth token`, then the environment.
func Token() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		if t := strings.TrimSpace(string(out)); t != "" {
			return t, nil
		}
	}
	if t := tokenFromEnv(os.Getenv); t != "" {
		return t, nil
	}
	return "", fmt.Errorf("no GitHub token found; run `gh auth login` or set GH_TOKEN")
}
