// Package config resolves which GitHub repository pr-tree operates on.
package config

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Repo identifies a GitHub repository.
type Repo struct {
	Owner string
	Name  string
}

func (r Repo) String() string { return r.Owner + "/" + r.Name }

// ownerName matches "owner/name", tolerating a trailing ".git".
var ownerName = regexp.MustCompile(`^([^/\s]+)/([^/\s]+?)(?:\.git)?$`)

// ParseOwnerName parses an "owner/name" string (the --repo flag value).
func ParseOwnerName(s string) (Repo, error) {
	m := ownerName.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Repo{}, fmt.Errorf("invalid repo %q, expected owner/name", s)
	}
	return Repo{Owner: m[1], Name: m[2]}, nil
}

// remoteURL matches a github.com remote in ssh, scp, or https form.
var remoteURL = regexp.MustCompile(`github\.com[:/]([^/]+)/(.+?)(?:\.git)?$`)

// ParseRemoteURL extracts owner/name from a git remote URL.
func ParseRemoteURL(url string) (Repo, error) {
	m := remoteURL.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return Repo{}, fmt.Errorf("could not parse GitHub repo from remote %q", url)
	}
	return Repo{Owner: m[1], Name: m[2]}, nil
}

// Resolve determines the target repo: the --repo flag when set, otherwise the
// current directory's git origin remote.
func Resolve(repoFlag string) (Repo, error) {
	if strings.TrimSpace(repoFlag) != "" {
		return ParseOwnerName(repoFlag)
	}
	out, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return Repo{}, fmt.Errorf("no --repo given and no git origin found; pass --repo owner/name")
	}
	return ParseRemoteURL(string(out))
}
